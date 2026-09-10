package service

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/google/uuid"
)

const (
	defaultSubPoolProbationDays = 7

	// subPoolGraduationScanInterval keeps the job cheap: probation is measured in
	// days, so scanning more often than this buys nothing.
	subPoolGraduationScanInterval = 10 * time.Minute
	// subPoolGraduationBatchSize caps one cycle so a backlog drains gradually
	// instead of dumping hundreds of keys into the formal pools at once.
	subPoolGraduationBatchSize = 50

	subPoolGraduationLeaderLockKey = "jobs:sub-pool-graduation"
)

// SubPoolProbationCandidate is a key that has served its time in a probe pool.
// Whether it actually graduates still depends on its behaviour during that time.
type SubPoolProbationCandidate struct {
	APIKeyID  int64
	GroupID   int64
	SubPoolID int64
	BoundAt   time.Time
}

// SubPoolGraduationPolicy is the admin-tunable half of the graduation rule.
type SubPoolGraduationPolicy struct {
	Enabled       bool
	ProbationDays int
	MaxDailyCalls int
}

// subPoolGraduationSettings is the slice of SettingService this job needs.
// Keeping it narrow means the job can be unit-tested without a settings stack.
type subPoolGraduationSettings interface {
	GetSubPoolGraduationPolicy(ctx context.Context) SubPoolGraduationPolicy
}

// GetSubPoolGraduationPolicy reads the probation rules. Every fallback is the
// conservative one: an unreadable setting leaves the job switched off rather
// than promoting keys onto production accounts on a guess.
func (s *SettingService) GetSubPoolGraduationPolicy(ctx context.Context) SubPoolGraduationPolicy {
	policy := SubPoolGraduationPolicy{ProbationDays: defaultSubPoolProbationDays}
	if s == nil || s.settingRepo == nil {
		return policy
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeySubPoolGraduationEnabled,
		SettingKeySubPoolProbationDays,
		SettingKeySubPoolProbationMaxDailyCalls,
	})
	if err != nil {
		slog.Warn("sub_pool_graduation_settings_unreadable", "error", err)
		return policy
	}
	policy.Enabled = strings.TrimSpace(values[SettingKeySubPoolGraduationEnabled]) == "true"
	if days := parsePositiveSettingInt(values[SettingKeySubPoolProbationDays]); days > 0 {
		policy.ProbationDays = days
	}
	policy.MaxDailyCalls = parsePositiveSettingInt(values[SettingKeySubPoolProbationMaxDailyCalls])
	return policy
}

// SetSubPoolGraduationPolicy persists the probation rules. These live on the
// sub-pool admin API rather than the global settings page because they only
// mean anything to groups that run sub-pools.
func (s *SettingService) SetSubPoolGraduationPolicy(ctx context.Context, policy SubPoolGraduationPolicy) error {
	if s == nil || s.settingRepo == nil {
		return errors.New("setting repository is not configured")
	}
	if policy.ProbationDays <= 0 {
		policy.ProbationDays = defaultSubPoolProbationDays
	}
	if policy.MaxDailyCalls < 0 {
		policy.MaxDailyCalls = 0
	}
	return s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeySubPoolGraduationEnabled:      strconv.FormatBool(policy.Enabled),
		SettingKeySubPoolProbationDays:          strconv.Itoa(policy.ProbationDays),
		SettingKeySubPoolProbationMaxDailyCalls: strconv.Itoa(policy.MaxDailyCalls),
	})
}

func parsePositiveSettingInt(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// SubPoolGraduationService moves keys out of the probe pool once they have
// completed probation without incident.
//
// It is deliberately a background job rather than a check on the request path:
// graduation reads moderation history and daily usage aggregates, which is far
// too expensive to do per request, and being a few minutes late costs nothing.
type SubPoolGraduationService struct {
	pools      SubPoolRepository
	usage      SubPoolUsageRepository
	settings   subPoolGraduationSettings
	authCache  APIKeyAuthCacheInvalidator
	leaderLock LeaderLockCache

	ctx    context.Context
	cancel context.CancelFunc
	owner  string
	done   chan struct{}
}

func NewSubPoolGraduationService(
	pools SubPoolRepository,
	usage SubPoolUsageRepository,
	settings subPoolGraduationSettings,
	leaderLock LeaderLockCache,
) *SubPoolGraduationService {
	ctx, cancel := context.WithCancel(context.Background())
	return &SubPoolGraduationService{
		pools:      pools,
		usage:      usage,
		settings:   settings,
		leaderLock: leaderLock,
		ctx:        ctx,
		cancel:     cancel,
		owner:      uuid.NewString(),
		done:       make(chan struct{}),
	}
}

// SetAuthCacheInvalidator is optional wiring, kept out of the constructor to
// avoid an import cycle with APIKeyService. Without it a graduated key keeps
// scheduling against its probe pool until the auth snapshot expires.
func (s *SubPoolGraduationService) SetAuthCacheInvalidator(invalidator APIKeyAuthCacheInvalidator) {
	if s == nil {
		return
	}
	s.authCache = invalidator
}

func (s *SubPoolGraduationService) Start() {
	if s == nil || s.pools == nil || s.settings == nil {
		return
	}
	go s.run()
}

func (s *SubPoolGraduationService) Stop() {
	if s == nil {
		return
	}
	s.cancel()
	select {
	case <-s.done:
	case <-time.After(5 * time.Second):
	}
}

func (s *SubPoolGraduationService) run() {
	defer close(s.done)
	ticker := time.NewTicker(subPoolGraduationScanInterval)
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

// RunOnce performs a single graduation sweep and returns how many keys moved.
// It is exported so an admin can trigger it on demand and so tests do not have
// to wait for the ticker.
func (s *SubPoolGraduationService) RunOnce(ctx context.Context) int {
	policy := s.settings.GetSubPoolGraduationPolicy(ctx)
	if !policy.Enabled {
		return 0
	}

	release, ok := s.tryAcquireLock(ctx)
	if !ok {
		return 0
	}
	if release != nil {
		defer release()
	}

	boundBefore := timezone.Now().AddDate(0, 0, -policy.ProbationDays)
	candidates, err := s.pools.ListProbationCandidates(ctx, boundBefore, subPoolGraduationBatchSize)
	if err != nil {
		slog.Warn("sub_pool_graduation_candidates_failed", "error", err)
		return 0
	}

	// Pool lists are cached per group: a group with 30 probation keys should not
	// trigger 30 identical queries, and every graduation mutates BoundKeys on the
	// target so capacity is respected within the sweep.
	poolsByGroup := make(map[int64][]SubPool)
	touchedGroups := make(map[int64]struct{})
	graduated := 0

	for _, candidate := range candidates {
		if ctx.Err() != nil {
			break
		}
		if eligible, reason := s.isEligible(ctx, candidate, policy); !eligible {
			slog.Debug("sub_pool_graduation_skipped",
				"api_key_id", candidate.APIKeyID, "sub_pool_id", candidate.SubPoolID, "reason", reason)
			continue
		}

		pools, ok := poolsByGroup[candidate.GroupID]
		if !ok {
			pools, err = s.pools.ListByGroup(ctx, candidate.GroupID)
			if err != nil {
				slog.Warn("sub_pool_graduation_pools_failed", "group_id", candidate.GroupID, "error", err)
				continue
			}
			poolsByGroup[candidate.GroupID] = pools
		}

		target := pickGraduationTarget(pools, candidate.SubPoolID)
		if target == nil {
			// No formal pool has room. Leaving the key in probation is the right
			// outcome: overfilling a good pool is how one bad key burns everyone.
			slog.Info("sub_pool_graduation_no_target",
				"api_key_id", candidate.APIKeyID, "group_id", candidate.GroupID)
			continue
		}
		if err := s.pools.BindKey(ctx, SubPoolBindInput{
			APIKeyID:  candidate.APIKeyID,
			SubPoolID: target.ID,
			GroupID:   candidate.GroupID,
			Reason:    domain.SubPoolBindReasonProbeGraduation,
			Operator:  domain.SubPoolBindOperatorSystem,
		}); err != nil {
			slog.Warn("sub_pool_graduation_bind_failed",
				"api_key_id", candidate.APIKeyID, "sub_pool_id", target.ID, "error", err)
			continue
		}
		target.BoundKeys++
		touchedGroups[candidate.GroupID] = struct{}{}
		graduated++
		slog.Info("sub_pool_key_graduated",
			"api_key_id", candidate.APIKeyID,
			"from_sub_pool_id", candidate.SubPoolID,
			"to_sub_pool_id", target.ID,
			"probation_days", policy.ProbationDays)
	}

	// The bound pool is part of the cached auth snapshot, so without this the key
	// would keep scheduling against its probe accounts until the snapshot aged out.
	if s.authCache != nil {
		for groupID := range touchedGroups {
			s.authCache.InvalidateAuthCacheByGroupID(ctx, groupID)
		}
	}
	return graduated
}

// isEligible applies the behavioural half of the rule. The candidate query has
// already established that probation is over.
func (s *SubPoolGraduationService) isEligible(
	ctx context.Context,
	candidate SubPoolProbationCandidate,
	policy SubPoolGraduationPolicy,
) (bool, string) {
	if s.usage == nil {
		return true, ""
	}

	violations, err := s.usage.CountViolationsSince(ctx, candidate.APIKeyID, candidate.BoundAt)
	if err != nil {
		// Promoting a key whose record we could not read would defeat the point
		// of the probe pool, so a failed lookup keeps it where it is.
		return false, "violation_lookup_failed"
	}
	if violations > 0 {
		return false, "violations"
	}

	if policy.MaxDailyCalls > 0 {
		peak, err := s.usage.MaxDailyCallsSince(ctx, candidate.APIKeyID, candidate.BoundAt)
		if err != nil {
			return false, "usage_lookup_failed"
		}
		if peak > int64(policy.MaxDailyCalls) {
			return false, "usage_above_cap"
		}
	}
	return true, ""
}

// pickGraduationTarget returns the emptiest healthy formal pool with accounts.
// Probe pools are never a target: graduating from one probe pool into another
// would just restart the clock.
func pickGraduationTarget(pools []SubPool, excludeID int64) *SubPool {
	var best *SubPool
	for i := range pools {
		pool := &pools[i]
		if pool.ID == excludeID || pool.IsProbe() {
			continue
		}
		if !pool.AcceptsNewBindings() || len(pool.AccountIDs) == 0 {
			continue
		}
		if best == nil || pool.BoundKeys < best.BoundKeys {
			best = pool
		}
	}
	return best
}

func (s *SubPoolGraduationService) tryAcquireLock(ctx context.Context) (func(), bool) {
	if s.leaderLock == nil {
		return func() {}, true
	}
	ok, err := s.leaderLock.TryAcquireLeaderLock(ctx, subPoolGraduationLeaderLockKey, s.owner, 5*time.Minute)
	if err != nil {
		// A flaky Redis must not stall graduation forever. Duplicate sweeps are
		// harmless: a key already moved out of the probe pool no longer matches
		// the candidate query.
		slog.Warn("sub_pool_graduation_leader_lock_unavailable", "error", err)
		return func() {}, true
	}
	if !ok {
		return nil, false
	}
	return func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.leaderLock.ReleaseLeaderLock(releaseCtx, subPoolGraduationLeaderLockKey, s.owner)
	}, true
}
