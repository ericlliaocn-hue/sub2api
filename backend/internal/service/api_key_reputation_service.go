package service

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/google/uuid"
)

const (
	defaultReputationWindowDays  = 30
	defaultReputationDemoteBelow = 60
	defaultReputationBanBelow    = 25

	// reputationScanInterval is slower than the cooling sweep on purpose: this
	// job hands out bans, and reacting within minutes buys nothing when the
	// window is measured in weeks.
	reputationScanInterval = 5 * time.Minute

	reputationLeaderLockKey = "jobs:api-key-reputation"
)

// ReputationPolicy is the admin-tunable half of the scoring rule.
type ReputationPolicy struct {
	Enabled     bool `json:"enabled"`
	WindowDays  int  `json:"window_days"`
	DemoteBelow int  `json:"demote_below"`
	BanBelow    int  `json:"ban_below"`
}

// Sub-pool reputation setting keys.
const (
	SettingKeyKeyReputationEnabled     = "key_reputation_enabled"
	SettingKeyKeyReputationWindowDays  = "key_reputation_window_days"
	SettingKeyKeyReputationDemoteBelow = "key_reputation_demote_below"
	SettingKeyKeyReputationBanBelow    = "key_reputation_ban_below"
)

// GetReputationPolicy reads the scoring thresholds. Like the graduation policy,
// every fallback leaves the feature off rather than guessing at a ban threshold.
func (s *SettingService) GetReputationPolicy(ctx context.Context) ReputationPolicy {
	policy := ReputationPolicy{
		WindowDays:  defaultReputationWindowDays,
		DemoteBelow: defaultReputationDemoteBelow,
		BanBelow:    defaultReputationBanBelow,
	}
	if s == nil || s.settingRepo == nil {
		return policy
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyKeyReputationEnabled,
		SettingKeyKeyReputationWindowDays,
		SettingKeyKeyReputationDemoteBelow,
		SettingKeyKeyReputationBanBelow,
	})
	if err != nil {
		slog.Warn("key_reputation_settings_unreadable", "error", err)
		return policy
	}
	policy.Enabled = strings.TrimSpace(values[SettingKeyKeyReputationEnabled]) == "true"
	if days := parsePositiveSettingInt(values[SettingKeyKeyReputationWindowDays]); days > 0 {
		policy.WindowDays = days
	}
	if v := parsePositiveSettingInt(values[SettingKeyKeyReputationDemoteBelow]); v > 0 {
		policy.DemoteBelow = v
	}
	if v := parsePositiveSettingInt(values[SettingKeyKeyReputationBanBelow]); v > 0 {
		policy.BanBelow = v
	}
	return policy
}

// SetReputationPolicy persists the scoring thresholds.
func (s *SettingService) SetReputationPolicy(ctx context.Context, policy ReputationPolicy) error {
	if s == nil || s.settingRepo == nil {
		return errors.New("setting repository is not configured")
	}
	if policy.WindowDays <= 0 {
		policy.WindowDays = defaultReputationWindowDays
	}
	if policy.DemoteBelow <= 0 {
		policy.DemoteBelow = defaultReputationDemoteBelow
	}
	if policy.BanBelow <= 0 {
		policy.BanBelow = defaultReputationBanBelow
	}
	// A ban threshold at or above the demote threshold would make demotion
	// unreachable: every key bad enough to demote would already be banned.
	if policy.BanBelow >= policy.DemoteBelow {
		return infraerrors.BadRequest("REPUTATION_THRESHOLDS_INVALID",
			"ban threshold must be lower than the demote threshold")
	}
	return s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyKeyReputationEnabled:     strconv.FormatBool(policy.Enabled),
		SettingKeyKeyReputationWindowDays:  strconv.Itoa(policy.WindowDays),
		SettingKeyKeyReputationDemoteBelow: strconv.Itoa(policy.DemoteBelow),
		SettingKeyKeyReputationBanBelow:    strconv.Itoa(policy.BanBelow),
	})
}

// reputationSettings is the narrow slice of SettingService this job needs.
type reputationSettings interface {
	GetReputationPolicy(ctx context.Context) ReputationPolicy
}

// reputationSanctioner is the Phase C sanction surface, injected as an interface
// so the job can be tested without a sub-pool stack.
type reputationSanctioner interface {
	DemoteToProbe(ctx context.Context, apiKeyID int64, operator string, note *string) error
	DisableKey(ctx context.Context, apiKeyID int64, operator string) error
}

// APIKeyReputationService turns moderation history into a per-key score and
// applies the sanctions the score earns.
//
// It sits below the existing content-moderation auto-ban, which disables the
// *user* after enough flagged hits. This one acts on the *key*, which is the
// lower rung of the escalation ladder: rate limit, demote to probe, ban key,
// then ban user.
type APIKeyReputationService struct {
	repo       APIKeyReputationRepository
	settings   reputationSettings
	sanctioner reputationSanctioner
	leaderLock LeaderLockCache

	ctx    context.Context
	cancel context.CancelFunc
	owner  string
	done   chan struct{}
}

func NewAPIKeyReputationService(
	repo APIKeyReputationRepository,
	settings reputationSettings,
	sanctioner reputationSanctioner,
	leaderLock LeaderLockCache,
) *APIKeyReputationService {
	ctx, cancel := context.WithCancel(context.Background())
	return &APIKeyReputationService{
		repo:       repo,
		settings:   settings,
		sanctioner: sanctioner,
		leaderLock: leaderLock,
		ctx:        ctx,
		cancel:     cancel,
		owner:      uuid.NewString(),
		done:       make(chan struct{}),
	}
}

func (s *APIKeyReputationService) Start() {
	if s == nil || s.repo == nil || s.settings == nil {
		return
	}
	go s.run()
}

func (s *APIKeyReputationService) Stop() {
	if s == nil {
		return
	}
	s.cancel()
	select {
	case <-s.done:
	case <-time.After(5 * time.Second):
	}
}

func (s *APIKeyReputationService) run() {
	defer close(s.done)
	ticker := time.NewTicker(reputationScanInterval)
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

// ReputationSweepResult summarises one scoring pass.
type ReputationSweepResult struct {
	Scored   int `json:"scored"`
	Demoted  int `json:"demoted"`
	Disabled int `json:"disabled"`
}

// RunOnce rescores every key with recent hits and applies sanctions.
func (s *APIKeyReputationService) RunOnce(ctx context.Context) ReputationSweepResult {
	var result ReputationSweepResult

	policy := s.settings.GetReputationPolicy(ctx)
	if !policy.Enabled {
		return result
	}

	release, ok := s.tryAcquireLock(ctx)
	if !ok {
		return result
	}
	if release != nil {
		defer release()
	}

	window := time.Duration(policy.WindowDays) * 24 * time.Hour
	since := timezone.Now().Add(-window)
	penalties, err := s.repo.AggregatePenalties(ctx, since, window)
	if err != nil {
		slog.Warn("key_reputation_aggregate_failed", "error", err)
		return result
	}

	scored := make([]int64, 0, len(penalties))
	now := timezone.Now()
	for _, penalty := range penalties {
		if ctx.Err() != nil {
			break
		}
		rep := &APIKeyReputation{
			APIKeyID:    penalty.APIKeyID,
			Score:       reputationScore(penalty.Penalty),
			SevereHits:  penalty.SevereHits,
			TotalHits:   penalty.TotalHits,
			LastEventAt: penalty.LastEventAt,
			ScoredAt:    now,
		}
		if err := s.repo.Upsert(ctx, rep); err != nil {
			slog.Warn("key_reputation_upsert_failed", "api_key_id", rep.APIKeyID, "error", err)
			continue
		}
		scored = append(scored, rep.APIKeyID)
		result.Scored++

		switch s.applySanction(ctx, rep, policy) {
		case ReputationSanctionDisabled:
			result.Disabled++
		case ReputationSanctionDemoted:
			result.Demoted++
		}
	}

	// Keys whose last hit has aged out of the window go back to a clean score.
	if err := s.repo.ResetScoresNotIn(ctx, scored); err != nil {
		slog.Warn("key_reputation_reset_failed", "error", err)
	}

	if result.Scored > 0 {
		slog.Info("key_reputation_sweep",
			"scored", result.Scored, "demoted", result.Demoted, "disabled", result.Disabled)
	}
	return result
}

// reputationScore converts a penalty into a 0-100 score.
func reputationScore(penalty int) int {
	score := reputationMaxScore - penalty
	if score < 0 {
		return 0
	}
	if score > reputationMaxScore {
		return reputationMaxScore
	}
	return score
}

// applySanction escalates a key at most one step per sweep and records what it
// did, so a key that stays below the threshold is not re-sanctioned every cycle.
// It returns the sanction applied, or ReputationSanctionNone.
func (s *APIKeyReputationService) applySanction(
	ctx context.Context,
	rep *APIKeyReputation,
	policy ReputationPolicy,
) string {
	if s.sanctioner == nil {
		return ReputationSanctionNone
	}

	current, err := s.repo.Get(ctx, rep.APIKeyID)
	if err != nil || current == nil {
		return ReputationSanctionNone
	}
	if current.Sanction == ReputationSanctionDisabled {
		return ReputationSanctionNone
	}

	switch {
	case rep.Score < policy.BanBelow:
		reason := reputationSanctionReason("score below ban threshold", rep.Score, policy.BanBelow)
		if err := s.sanctioner.DisableKey(ctx, rep.APIKeyID, "system:reputation"); err != nil {
			slog.Warn("key_reputation_ban_failed", "api_key_id", rep.APIKeyID, "error", err)
			return ReputationSanctionNone
		}
		if err := s.repo.MarkSanctioned(ctx, rep.APIKeyID, ReputationSanctionDisabled, reason); err != nil {
			slog.Warn("key_reputation_mark_failed", "api_key_id", rep.APIKeyID, "error", err)
		}
		slog.Info("key_reputation_key_banned", "api_key_id", rep.APIKeyID, "score", rep.Score)
		return ReputationSanctionDisabled

	case rep.Score < policy.DemoteBelow && current.Sanction == ReputationSanctionNone:
		reason := reputationSanctionReason("score below demote threshold", rep.Score, policy.DemoteBelow)
		if err := s.sanctioner.DemoteToProbe(ctx, rep.APIKeyID, "system:reputation", &reason); err != nil {
			// A group without sub-pools has nowhere to demote to. That is a
			// configuration fact, not a failure: the score is still recorded and
			// the key will be banned if it keeps sliding.
			slog.Info("key_reputation_demote_skipped", "api_key_id", rep.APIKeyID, "error", err)
			return ReputationSanctionNone
		}
		if err := s.repo.MarkSanctioned(ctx, rep.APIKeyID, ReputationSanctionDemoted, reason); err != nil {
			slog.Warn("key_reputation_mark_failed", "api_key_id", rep.APIKeyID, "error", err)
		}
		slog.Info("key_reputation_key_demoted", "api_key_id", rep.APIKeyID, "score", rep.Score)
		return ReputationSanctionDemoted
	}
	return ReputationSanctionNone
}

func reputationSanctionReason(what string, score, threshold int) string {
	return what + ": " + strconv.Itoa(score) + " < " + strconv.Itoa(threshold)
}

// Get returns one key's reputation for the admin console.
func (s *APIKeyReputationService) Get(ctx context.Context, apiKeyID int64) (*APIKeyReputation, error) {
	return s.repo.Get(ctx, apiKeyID)
}

// ListWorst returns the lowest-scoring keys.
func (s *APIKeyReputationService) ListWorst(ctx context.Context, limit int) ([]APIKeyReputation, error) {
	return s.repo.ListWorst(ctx, limit)
}

// ClearSanction overturns an automatic decision. Section 5 of the plan requires
// this: a key wrongly held back must be reinstatable by an operator.
func (s *APIKeyReputationService) ClearSanction(ctx context.Context, apiKeyID int64) error {
	return s.repo.ClearSanction(ctx, apiKeyID)
}

func (s *APIKeyReputationService) tryAcquireLock(ctx context.Context) (func(), bool) {
	if s.leaderLock == nil {
		return func() {}, true
	}
	ok, err := s.leaderLock.TryAcquireLeaderLock(ctx, reputationLeaderLockKey, s.owner, 4*time.Minute)
	if err != nil {
		slog.Warn("key_reputation_leader_lock_unavailable", "error", err)
		return func() {}, true
	}
	if !ok {
		return nil, false
	}
	return func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.leaderLock.ReleaseLeaderLock(releaseCtx, reputationLeaderLockKey, s.owner)
	}, true
}
