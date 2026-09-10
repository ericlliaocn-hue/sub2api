package service

import (
	"context"
	"log/slog"
	"strconv"
	"time"

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

// AllowedAccountIDs returns the account set backing the pool. The second return
// value is false when membership could not be resolved, in which case callers
// must leave the candidate list untouched rather than guess.
func (m *SubPoolMembership) AllowedAccountIDs(ctx context.Context, subPoolID int64) (map[int64]struct{}, bool) {
	if m == nil || m.repo == nil || subPoolID <= 0 {
		return nil, false
	}
	key := strconv.FormatInt(subPoolID, 10)
	if cached, ok := m.cache.Get(key); ok {
		if allowed, ok := cached.(map[int64]struct{}); ok {
			return allowed, true
		}
	}
	result, err, _ := m.sf.Do(key, func() (any, error) {
		ids, err := m.repo.ListAccountIDs(ctx, subPoolID)
		if err != nil {
			return nil, err
		}
		allowed := make(map[int64]struct{}, len(ids))
		for _, id := range ids {
			allowed[id] = struct{}{}
		}
		m.cache.Set(key, allowed, subPoolMembershipTTL)
		return allowed, nil
	})
	if err != nil {
		return nil, false
	}
	allowed, ok := result.(map[int64]struct{})
	return allowed, ok
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
// An empty result is deliberate: if the pool has no schedulable account left,
// the request fails with "no available accounts" instead of spilling over to
// the rest of the group. Spilling over would hand the abusing key a fresh set
// of accounts to burn, which is exactly what the pool exists to prevent.
func (m *SubPoolMembership) FilterAccountsBySubPool(ctx context.Context, accounts []Account) []Account {
	subPoolID := SubPoolIDFromContext(ctx)
	if m == nil || subPoolID <= 0 || len(accounts) == 0 {
		return accounts
	}
	allowed, ok := m.AllowedAccountIDs(ctx, subPoolID)
	if !ok {
		slog.Warn("sub_pool_membership_unresolved_scheduling_unrestricted",
			"sub_pool_id", subPoolID, "candidates", len(accounts))
		return accounts
	}
	filtered := make([]Account, 0, len(accounts))
	for _, acc := range accounts {
		if _, in := allowed[acc.ID]; in {
			filtered = append(filtered, acc)
		}
	}
	slog.Debug("sub_pool_scheduling_filter",
		"sub_pool_id", subPoolID,
		"candidates", len(accounts),
		"allowed", len(filtered))
	return filtered
}
