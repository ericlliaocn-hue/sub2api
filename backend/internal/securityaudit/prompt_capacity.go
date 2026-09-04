package securityaudit

import (
	"context"
	"sort"
	"strings"
	"sync"
)

const (
	defaultPromptSyncGlobalLimit  = 16
	defaultPromptSyncNodeLimit    = 4
	defaultPromptAsyncGlobalLimit = 1
	defaultPromptAsyncNodeLimit   = 1
)

type promptCapacityNode struct {
	active      int
	asyncActive int
}

// promptCapacity coordinates synchronous and background model calls. Background
// work is deliberately low priority: it never waits for capacity and can hold
// at most one global and one per-node slot.
type promptCapacity struct {
	mu sync.Mutex

	notify chan struct{}
	nodes  map[string]*promptCapacityNode

	globalLimit      int
	nodeLimit        int
	asyncGlobalLimit int
	asyncNodeLimit   int
	asyncGlobalCap   int
	asyncNodeCap     int

	globalActive      int
	asyncGlobalActive int
}

func newPromptCapacity(globalLimit, nodeLimit, asyncGlobalLimit, asyncNodeLimit int) *promptCapacity {
	if globalLimit < 1 {
		globalLimit = defaultPromptSyncGlobalLimit
	}
	if nodeLimit < 1 {
		nodeLimit = defaultPromptSyncNodeLimit
	}
	if asyncGlobalLimit < 1 {
		asyncGlobalLimit = defaultPromptAsyncGlobalLimit
	}
	if asyncNodeLimit < 1 {
		asyncNodeLimit = defaultPromptAsyncNodeLimit
	}
	return &promptCapacity{
		notify: make(chan struct{}), nodes: make(map[string]*promptCapacityNode),
		globalLimit: globalLimit, nodeLimit: nodeLimit,
		asyncGlobalLimit: asyncGlobalLimit, asyncNodeLimit: asyncNodeLimit,
		asyncGlobalCap: reservedAsyncCapacity(globalLimit), asyncNodeCap: reservedAsyncCapacity(nodeLimit),
	}
}

func reservedAsyncCapacity(limit int) int {
	if limit > 1 {
		return limit - 1
	}
	return limit
}

func (c *promptCapacity) AcquireSync(ctx context.Context, nodeID string) (func(), bool) {
	if c == nil {
		return func() {}, true
	}
	if ctx == nil {
		ctx = context.Background()
	}
	nodeID = normalizePromptCapacityNodeID(nodeID)
	for {
		c.mu.Lock()
		node := c.nodeLocked(nodeID)
		if c.globalActive < c.globalLimit && node.active < c.nodeLimit {
			c.globalActive++
			node.active++
			c.mu.Unlock()
			return c.releaseFunc(nodeID, node, false), true
		}
		notify := c.notify
		c.mu.Unlock()

		select {
		case <-notify:
		case <-ctx.Done():
			return nil, false
		}
	}
}

func (c *promptCapacity) TryAcquireAsync(nodeID string) (func(), bool) {
	if c == nil {
		return func() {}, true
	}
	nodeID = normalizePromptCapacityNodeID(nodeID)
	c.mu.Lock()
	defer c.mu.Unlock()
	node := c.nodeLocked(nodeID)
	if c.globalActive >= c.asyncGlobalCap || node.active >= c.asyncNodeCap ||
		c.asyncGlobalActive >= c.asyncGlobalLimit || node.asyncActive >= c.asyncNodeLimit {
		c.cleanupNodeLocked(nodeID, node)
		return nil, false
	}
	c.globalActive++
	c.asyncGlobalActive++
	node.active++
	node.asyncActive++
	return c.releaseFunc(nodeID, node, true), true
}

// OrderEndpoints returns a detached endpoint slice ordered for the configured
// selection strategy. Least-inflight is deliberately based on local admission
// state, so a slow or unreachable node naturally stops attracting new work
// after its active slots fill. Stable sorting preserves configured priority for
// ties and for the legacy priority strategy.
func (c *promptCapacity) OrderEndpoints(strategy string, endpoints []ActiveEndpoint) []ActiveEndpoint {
	ordered := append([]ActiveEndpoint(nil), endpoints...)
	if c == nil || strings.TrimSpace(strategy) != PromptAuditStrategyLeastInflight || len(ordered) < 2 {
		return ordered
	}
	loads := make(map[string]int, len(ordered))
	c.mu.Lock()
	for _, endpoint := range ordered {
		if node := c.nodes[normalizePromptCapacityNodeID(endpoint.ID)]; node != nil {
			loads[endpoint.ID] = node.active
		}
	}
	c.mu.Unlock()
	sort.SliceStable(ordered, func(i, j int) bool {
		return loads[ordered[i].ID] < loads[ordered[j].ID]
	})
	return ordered
}

func (c *promptCapacity) nodeLocked(nodeID string) *promptCapacityNode {
	nodeID = normalizePromptCapacityNodeID(nodeID)
	node := c.nodes[nodeID]
	if node == nil {
		node = &promptCapacityNode{}
		c.nodes[nodeID] = node
	}
	return node
}

func (c *promptCapacity) releaseFunc(nodeID string, node *promptCapacityNode, async bool) func() {
	nodeID = normalizePromptCapacityNodeID(nodeID)
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			if c.globalActive > 0 {
				c.globalActive--
			}
			if node.active > 0 {
				node.active--
			}
			if async {
				if c.asyncGlobalActive > 0 {
					c.asyncGlobalActive--
				}
				if node.asyncActive > 0 {
					node.asyncActive--
				}
			}
			close(c.notify)
			c.notify = make(chan struct{})
			c.cleanupNodeLocked(nodeID, node)
			c.mu.Unlock()
		})
	}
}

func (c *promptCapacity) cleanupNodeLocked(nodeID string, node *promptCapacityNode) {
	nodeID = normalizePromptCapacityNodeID(nodeID)
	if node.active == 0 && node.asyncActive == 0 && c.nodes[nodeID] == node {
		delete(c.nodes, nodeID)
	}
}

func normalizePromptCapacityNodeID(nodeID string) string {
	if nodeID == "" {
		return "default"
	}
	return nodeID
}
