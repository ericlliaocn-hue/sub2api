package service

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	gocache "github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

// subPoolMembershipTTL keeps the scheduling hot path off the database. Pool
// membership only changes through admin actions, so a few seconds of staleness
// is acceptable; SetAccounts invalidates the entry immediately anyway.
const subPoolMembershipTTL = 15 * time.Second

// SubPoolMembership resolves "which upstream accounts may this sub-pool use"
// for the scheduling hot path.
type SubPoolMembership struct {
	repo  SubPoolRepository
	cache *gocache.Cache
	sf    singleflight.Group
}

func NewSubPoolMembership(repo SubPoolRepository) *SubPoolMembership {
	return &SubPoolMembership{
		repo:  repo,
		cache: gocache.New(subPoolMembershipTTL, 2*subPoolMembershipTTL),
	}
}

// resolvedSubPool is the cached scheduling view of a pool.
type resolvedSubPool struct {
	status  string
	allowed map[int64]struct{}
}

// resolve returns the pool's scheduling state. The second return value is false
// when it could not be resolved, in which case callers must leave the candidate
// list untouched rather than guess.
func (m *SubPoolMembership) resolve(ctx context.Context, subPoolID int64) (*resolvedSubPool, bool) {
	if m == nil || m.repo == nil || subPoolID <= 0 {
		return nil, false
	}
	key := strconv.FormatInt(subPoolID, 10)
	if cached, ok := m.cache.Get(key); ok {
		if state, ok := cached.(*resolvedSubPool); ok {
			return state, true
		}
	}
	result, err, _ := m.sf.Do(key, func() (any, error) {
		state, err := m.repo.GetSchedulingState(ctx, subPoolID)
		if err != nil {
			return nil, err
		}
		resolved := &resolvedSubPool{
			status:  state.Status,
			allowed: make(map[int64]struct{}, len(state.AccountIDs)),
		}
		for _, id := range state.AccountIDs {
			resolved.allowed[id] = struct{}{}
		}
		m.cache.Set(key, resolved, subPoolMembershipTTL)
		return resolved, nil
	})
	if err != nil {
		return nil, false
	}
	resolved, ok := result.(*resolvedSubPool)
	return resolved, ok
}

// AllowedAccountIDs returns the account set backing the pool.
func (m *SubPoolMembership) AllowedAccountIDs(ctx context.Context, subPoolID int64) (map[int64]struct{}, bool) {
	state, ok := m.resolve(ctx, subPoolID)
	if !ok {
		return nil, false
	}
	return state.allowed, true
}

// Invalidate drops the cached membership of a pool after an admin change.
func (m *SubPoolMembership) Invalidate(subPoolID int64) {
	if m == nil {
		return
	}
	m.cache.Delete(strconv.FormatInt(subPoolID, 10))
}

// SubPoolIDFromContext reads the sub-pool the authenticated API key is bound
// to. It is only present when the key's group runs sub-pool scheduling.
func SubPoolIDFromContext(ctx context.Context) int64 {
	if ctx == nil {
		return 0
	}
	if id, ok := ctx.Value(ctxkey.SubPoolID).(int64); ok {
		return id
	}
	return 0
}

// FilterAccountsBySubPool narrows scheduling candidates to the accounts of the
// key's sub-pool. It is a no-op when the request carries no sub-pool.
//
// An empty result is deliberate: if the pool is cooling or has no schedulable
// account left,
// the request fails with "no available accounts" instead of spilling over to
// the rest of the group. Spilling over would hand the abusing key a fresh set
// of accounts to burn, which is exactly what the pool exists to prevent.
func (m *SubPoolMembership) FilterAccountsBySubPool(ctx context.Context, accounts []Account) []Account {
	subPoolID := SubPoolIDFromContext(ctx)
	if m == nil || subPoolID <= 0 || len(accounts) == 0 {
		return accounts
	}
	state, ok := m.resolve(ctx, subPoolID)
	if !ok {
		slog.Warn("sub_pool_membership_unresolved_scheduling_unrestricted",
			"sub_pool_id", subPoolID, "candidates", len(accounts))
		return accounts
	}
	// A cooling pool is one whose upstream accounts are burned or under
	// investigation. Draining it is the whole response to an incident, so it
	// must stop serving traffic and not merely stop accepting new keys.
	if state.status == domain.SubPoolStatusCooling {
		slog.Debug("sub_pool_scheduling_blocked_cooling",
			"sub_pool_id", subPoolID, "candidates", len(accounts))
		return nil
	}
	filtered := make([]Account, 0, len(accounts))
	for _, acc := range accounts {
		if _, in := state.allowed[acc.ID]; in {
			filtered = append(filtered, acc)
		}
	}
	slog.Debug("sub_pool_scheduling_filter",
		"sub_pool_id", subPoolID,
		"candidates", len(accounts),
		"allowed", len(filtered))
	return filtered
}
