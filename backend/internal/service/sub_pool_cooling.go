package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/google/uuid"
)

const (
	// subPoolCoolingScanInterval is tighter than the graduation sweep: a burned
	// pool is serving errors to real users until it is drained.
	subPoolCoolingScanInterval = 2 * time.Minute

	// subPoolCoolingMinDuration is hysteresis. Upstream accounts flap in and out
	// of rate limits constantly, and a pool that recovers the instant an account
	// looks healthy would move keys back and forth all day.
	subPoolCoolingMinDuration = 15 * time.Minute

	// subPoolAutoMigrationDebounce caps how often the system may move one key on
	// its own. Admin moves bypass this.
	subPoolAutoMigrationDebounce = 24 * time.Hour

	// subPoolMigrationsPerCycle rate-limits the drain so a widespread incident
	// does not stampede every key into the one remaining healthy pool at once.
	subPoolMigrationsPerCycle = 20

	subPoolCoolingLeaderLockKey = "jobs:sub-pool-cooling"
)

// subPoolAccountReader is the slice of AccountRepository the sweep needs.
type subPoolAccountReader interface {
	GetByIDs(ctx context.Context, ids []int64) ([]*Account, error)
}

// SubPoolCoolingService is the incident half of the sub-pool machinery: it puts
// pools whose upstream accounts died into cooling, drains the bystanders, and
// brings pools back once they recover.
type SubPoolCoolingService struct {
	pools      SubPoolRepository
	accounts   subPoolAccountReader
	subPools   *SubPoolService
	membership *SubPoolMembership
	authCache  APIKeyAuthCacheInvalidator
	leaderLock LeaderLockCache

	ctx    context.Context
	cancel context.CancelFunc
	owner  string
	done   chan struct{}
}

func NewSubPoolCoolingService(
	pools SubPoolRepository,
	accounts subPoolAccountReader,
	subPools *SubPoolService,
	membership *SubPoolMembership,
	leaderLock LeaderLockCache,
) *SubPoolCoolingService {
	ctx, cancel := context.WithCancel(context.Background())
	return &SubPoolCoolingService{
		pools:      pools,
		accounts:   accounts,
		subPools:   subPools,
		membership: membership,
		leaderLock: leaderLock,
		ctx:        ctx,
		cancel:     cancel,
		owner:      uuid.NewString(),
		done:       make(chan struct{}),
	}
}

func (s *SubPoolCoolingService) SetAuthCacheInvalidator(invalidator APIKeyAuthCacheInvalidator) {
	if s == nil {
		return
	}
	s.authCache = invalidator
}

func (s *SubPoolCoolingService) Start() {
	if s == nil || s.pools == nil || s.accounts == nil || s.subPools == nil {
		return
	}
	go s.run()
}

func (s *SubPoolCoolingService) Stop() {
	if s == nil {
		return
	}
	s.cancel()
	select {
	case <-s.done:
	case <-time.After(5 * time.Second):
	}
}

func (s *SubPoolCoolingService) run() {
	defer close(s.done)
	ticker := time.NewTicker(subPoolCoolingScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.RunOnce(s.ctx)
		}
	}
}

// SubPoolCoolingResult summarises one sweep for logging and the admin trigger.
type SubPoolCoolingResult struct {
	Cooled    int `json:"cooled"`
	Recovered int `json:"recovered"`
	Migrated  int `json:"migrated"`
}

// RunOnce performs one cooling sweep across every pool in a sub-pool group.
func (s *SubPoolCoolingService) RunOnce(ctx context.Context) SubPoolCoolingResult {
	var result SubPoolCoolingResult

	release, ok := s.tryAcquireLock(ctx)
	if !ok {
		return result
	}
	if release != nil {
		defer release()
	}

	pools, err := s.pools.ListPoolsInEnabledGroups(ctx)
	if err != nil {
		slog.Warn("sub_pool_cooling_list_failed", "error", err)
		return result
	}

	health, listed := s.accountHealth(ctx, pools)
	// 20x 兜底看整组还有没有主号：主号活着不准拉备用号；主号全死再拉，
	// 而且要扫整组（含已冷却的分流池），不能只看当前这个池。
	s.promoteGroupReserves(ctx, pools, listed, health)
	migrationBudget := subPoolMigrationsPerCycle

	for i := range pools {
		if ctx.Err() != nil {
			break
		}
		pool := &pools[i]
		switch pool.Status {
		case domain.SubPoolStatusHealthy:
			if poolHasUsableAccount(pool, health) {
				continue
			}
			migrated := s.cool(ctx, pool, &migrationBudget)
			result.Cooled++
			result.Migrated += migrated
		case domain.SubPoolStatusCooling:
			if s.recover(ctx, pool, health) {
				result.Recovered++
			}
		}
	}

	if result.Cooled > 0 || result.Recovered > 0 || result.Migrated > 0 {
		slog.Info("sub_pool_cooling_sweep",
			"cooled", result.Cooled, "recovered", result.Recovered, "migrated", result.Migrated)
	}
	return result
}

// accountHealth resolves every pool account in one query instead of one per pool.
func (s *SubPoolCoolingService) accountHealth(ctx context.Context, pools []SubPool) (map[int64]bool, []*Account) {
	ids := make([]int64, 0)
	seen := make(map[int64]struct{})
	for i := range pools {
		for _, id := range pools[i].AccountIDs {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	health := make(map[int64]bool, len(ids))
	if len(ids) == 0 {
		return health, nil
	}
	accounts, err := s.accounts.GetByIDs(ctx, ids)
	if err != nil {
		slog.Warn("sub_pool_cooling_accounts_failed", "error", err)
		// An unreadable account list must not be read as "every pool is dead".
		// Marking everything healthy makes the sweep a no-op this cycle.
		for _, id := range ids {
			health[id] = true
		}
		return health, nil
	}
	for _, account := range accounts {
		health[account.ID] = account.IsSchedulable()
	}
	return health, accounts
}

func groupAccountIDs(groupID int64, pools []SubPool) []int64 {
	seen := make(map[int64]struct{})
	ids := make([]int64, 0)
	for i := range pools {
		if pools[i].GroupID != groupID {
			continue
		}
		for _, id := range pools[i].AccountIDs {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids
}

func groupHasSchedulablePrimary(groupID int64, pools []SubPool, accounts []*Account) bool {
	ids := make(map[int64]struct{}, len(accounts))
	for _, id := range groupAccountIDs(groupID, pools) {
		ids[id] = struct{}{}
	}
	for _, account := range accounts {
		if account == nil {
			continue
		}
		if _, ok := ids[account.ID]; !ok {
			continue
		}
		if account.IsSchedulable() && !IsReserveFleetAccount(account) {
			return true
		}
	}
	return false
}

func (s *SubPoolCoolingService) promoteGroupReserves(ctx context.Context, pools []SubPool, listed []*Account, health map[int64]bool) {
	if s == nil {
		return
	}
	seen := make(map[int64]struct{})
	for i := range pools {
		groupID := pools[i].GroupID
		if _, ok := seen[groupID]; ok {
			continue
		}
		seen[groupID] = struct{}{}
		if groupHasSchedulablePrimary(groupID, pools, listed) {
			continue
		}
		s.promoteReserveAccounts(ctx, &SubPool{
			GroupID:    groupID,
			AccountIDs: groupAccountIDs(groupID, pools),
		}, health)
	}
}

// poolHasUsableAccount reports whether any account behind the pool can serve a
// request right now. A pool with no accounts at all is not "burned" — it is
// misconfigured, and cooling it would hide that.
type reserveSchedulableWriter interface {
	SetSchedulable(ctx context.Context, id int64, schedulable bool) error
}

func (s *SubPoolCoolingService) promoteReserveAccounts(ctx context.Context, pool *SubPool, health map[int64]bool) bool {
	if s == nil || s.accounts == nil || pool == nil || len(pool.AccountIDs) == 0 {
		return false
	}
	writer, ok := s.accounts.(reserveSchedulableWriter)
	if !ok {
		return false
	}
	accounts, err := s.accounts.GetByIDs(ctx, pool.AccountIDs)
	if err != nil {
		slog.Warn("sub_pool_reserve_promote_list_failed", "sub_pool_id", pool.ID, "error", err)
		return false
	}
	promoted := false
	for _, account := range accounts {
		if account == nil || !IsReserveFleetAccount(account) || account.Status != StatusActive || reserveAuthDead(account) {
			continue
		}
		if account.IsSchedulable() {
			health[account.ID] = true
			continue
		}
		if err := writer.SetSchedulable(ctx, account.ID, true); err != nil {
			slog.Warn("sub_pool_reserve_promote_failed", "account_id", account.ID, "error", err)
			continue
		}
		health[account.ID] = true
		promoted = true
		slog.Info("sub_pool_reserve_promoted", "account_id", account.ID, "sub_pool_id", pool.ID, "group_id", pool.GroupID)
	}
	return promoted
}

func poolHasUsableAccount(pool *SubPool, health map[int64]bool) bool {
	if len(pool.AccountIDs) == 0 {
		return true
	}
	for _, id := range pool.AccountIDs {
		if health[id] {
			return true
		}
	}
	return false
}

// cool moves the pool into cooling and drains the keys that are not suspects.
func (s *SubPoolCoolingService) cool(ctx context.Context, pool *SubPool, budget *int) int {
	until := timezone.Now().Add(subPoolCoolingMinDuration)
	reason := domain.SubPoolCoolingReasonAccountsUnavailable

	pool.Status = domain.SubPoolStatusCooling
	pool.CoolingUntil = &until
	pool.CoolingReason = &reason
	if err := s.pools.Update(ctx, pool); err != nil {
		slog.Warn("sub_pool_cooling_update_failed", "sub_pool_id", pool.ID, "error", err)
		return 0
	}
	s.membership.Invalidate(pool.ID)
	slog.Info("sub_pool_cooled", "sub_pool_id", pool.ID, "group_id", pool.GroupID, "reason", reason)

	return s.drain(ctx, pool, reason, budget)
}

// drain moves the clean keys out. Suspects stay: relocating them would hand
// them fresh accounts and detach them from the evidence.
func (s *SubPoolCoolingService) drain(ctx context.Context, pool *SubPool, reason string, budget *int) int {
	if *budget <= 0 {
		return 0
	}

	report, err := s.subPools.Attribution(ctx, pool.ID, subPoolAttributionWindow)
	if err != nil {
		slog.Warn("sub_pool_cooling_attribution_failed", "sub_pool_id", pool.ID, "error", err)
		return 0
	}
	suspects := make(map[int64]struct{})
	for _, id := range report.SuspectKeyIDs() {
		suspects[id] = struct{}{}
	}

	pools, err := s.pools.ListByGroup(ctx, pool.GroupID)
	if err != nil {
		slog.Warn("sub_pool_cooling_pools_failed", "group_id", pool.GroupID, "error", err)
		return 0
	}

	debounceSince := timezone.Now().Add(-subPoolAutoMigrationDebounce)
	moved := 0
	for _, row := range report.Keys {
		if *budget <= 0 || ctx.Err() != nil {
			break
		}
		if _, suspect := suspects[row.APIKeyID]; suspect {
			continue
		}
		// A key that the system already relocated today stays put: a pool that
		// keeps flapping would otherwise walk one user through every pool in the
		// group, and each hop resets their probation and peer history.
		recent, err := s.pools.CountAutoMigrationsSince(ctx, row.APIKeyID, debounceSince)
		if err != nil {
			slog.Warn("sub_pool_cooling_debounce_failed", "api_key_id", row.APIKeyID, "error", err)
			continue
		}
		if recent > 0 {
			slog.Info("sub_pool_migration_debounced", "api_key_id", row.APIKeyID, "sub_pool_id", pool.ID)
			continue
		}

		target := pickMigrationTarget(pools, pool.ID)
		if target == nil {
			slog.Info("sub_pool_cooling_no_target", "sub_pool_id", pool.ID, "group_id", pool.GroupID)
			break
		}
		note := reason
		if err := s.pools.BindKey(ctx, SubPoolBindInput{
			APIKeyID:  row.APIKeyID,
			SubPoolID: target.ID,
			GroupID:   pool.GroupID,
			Reason:    domain.SubPoolBindReasonCoolingMigration,
			Operator:  domain.SubPoolBindOperatorSystem,
			Note:      &note,
		}); err != nil {
			slog.Warn("sub_pool_cooling_bind_failed", "api_key_id", row.APIKeyID, "error", err)
			continue
		}
		target.BoundKeys++
		moved++
		*budget--
		slog.Info("sub_pool_clean_key_auto_migrated",
			"api_key_id", row.APIKeyID, "from_sub_pool_id", pool.ID, "to_sub_pool_id", target.ID)
	}

	if moved > 0 && s.authCache != nil {
		s.authCache.InvalidateAuthCacheByGroupID(ctx, pool.GroupID)
	}
	return moved
}

// recover returns a pool to service once its accounts are usable again and the
// minimum cooling period has elapsed.
func (s *SubPoolCoolingService) recover(ctx context.Context, pool *SubPool, health map[int64]bool) bool {
	// Only pools the state machine cooled are auto-recovered. An operator who
	// cooled a pool by hand gets to decide when it comes back.
	if pool.CoolingReason == nil || *pool.CoolingReason != domain.SubPoolCoolingReasonAccountsUnavailable {
		return false
	}
	// A newly attached or restored account should serve immediately. Waiting
	// out hysteresis left healthy cars idle while every key got pool=0.
	if !poolHasUsableAccount(pool, health) {
		return false
	}

	pool.Status = domain.SubPoolStatusHealthy
	pool.CoolingUntil = nil
	pool.CoolingReason = nil
	if err := s.pools.Update(ctx, pool); err != nil {
		slog.Warn("sub_pool_recover_update_failed", "sub_pool_id", pool.ID, "error", err)
		return false
	}
	s.membership.Invalidate(pool.ID)
	slog.Info("sub_pool_recovered", "sub_pool_id", pool.ID, "group_id", pool.GroupID)
	return true
}

func (s *SubPoolCoolingService) tryAcquireLock(ctx context.Context) (func(), bool) {
	if s.leaderLock == nil {
		return func() {}, true
	}
	ok, err := s.leaderLock.TryAcquireLeaderLock(ctx, subPoolCoolingLeaderLockKey, s.owner, 90*time.Second)
	if err != nil {
		slog.Warn("sub_pool_cooling_leader_lock_unavailable", "error", err)
		return func() {}, true
	}
	if !ok {
		return nil, false
	}
	return func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.leaderLock.ReleaseLeaderLock(releaseCtx, subPoolCoolingLeaderLockKey, s.owner)
	}, true
}
