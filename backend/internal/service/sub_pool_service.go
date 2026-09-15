package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

var (
	ErrSubPoolNoCapacity = infraerrors.Conflict("SUB_POOL_NO_CAPACITY",
		"no sub-pool in this group can accept a new key")
	ErrSubPoolNotEnabled = infraerrors.BadRequest("SUB_POOL_NOT_ENABLED",
		"sub-pool scheduling is not enabled for this group")
)

// SubPoolPeerWindow pins each key's aggregation start so a peer who joined
// yesterday is not credited with the traffic of whoever held the slot before.
type SubPoolPeerWindow struct {
	APIKeyID int64
	Since    time.Time
}

// SubPoolPeerActivity is the raw per-key aggregate behind the user-facing board.
type SubPoolPeerActivity struct {
	APIKeyID    int64
	FirstCallAt *time.Time
	LastCallAt  *time.Time
	TodayCalls  int64
	WeekCalls   int64
}

// SubPoolAccountKeyUsage attributes traffic on one upstream account to the keys
// that produced it.
type SubPoolAccountKeyUsage struct {
	APIKeyID    int64
	UserID      int64
	Calls       int64
	FirstCallAt *time.Time
	LastCallAt  *time.Time
}

// SubPoolUsageRepository is intentionally narrow: these queries serve pool
// attribution, not billing, and must not widen UsageLogRepository.
type SubPoolUsageRepository interface {
	PeerActivity(ctx context.Context, keys []SubPoolPeerWindow, todayStart, weekStart time.Time) (map[int64]SubPoolPeerActivity, error)
	TopKeysByAccount(ctx context.Context, accountID int64, start, end time.Time, limit int) ([]SubPoolAccountKeyUsage, error)

	// CountViolationsSince counts moderation and prompt-audit hits severe enough
	// to disqualify a key from leaving the probe pool.
	CountViolationsSince(ctx context.Context, apiKeyID int64, since time.Time) (int64, error)
	// MaxDailyCallsSince returns the busiest single day of the key since `since`,
	// bucketed by the site timezone.
	MaxDailyCallsSince(ctx context.Context, apiKeyID int64, since time.Time) (int64, error)
	// KeyUsageInWindow aggregates the given keys over one window. It backs the
	// "who burned this pool" report, so it must include keys with zero calls —
	// their absence from the traffic is itself evidence.
	KeyUsageInWindow(ctx context.Context, apiKeyIDs []int64, start, end time.Time) (map[int64]SubPoolAccountKeyUsage, error)
}

// SubPoolPeer is one anonymised member of the caller's own pool.
//
// It carries no identity at all — no email, user id, key name or IP. The point
// is to make shared-account behaviour visible ("someone here is hammering it"),
// not to expose who the neighbours are.
type SubPoolPeer struct {
	// Label is a stable per-pool alias such as "A"/"B", not a user reference.
	Label       string     `json:"label"`
	IsSelf      bool       `json:"is_self"`
	FirstCallAt *time.Time `json:"first_call_at"`
	LastCallAt  *time.Time `json:"last_call_at"`
	TodayCalls  int64      `json:"today_calls"`
	WeekCalls   int64      `json:"week_calls"`
}

// SubPoolPeerBoard is what a user sees about the pool behind their key.
type SubPoolPeerBoard struct {
	MemberCount int           `json:"member_count"`
	Peers       []SubPoolPeer `json:"peers"`
	// WeekWindowHours documents the rolling window so the UI need not guess.
	WeekWindowHours int `json:"week_window_hours"`
}

// SubPoolService owns pool lifecycle, key placement and the user-facing board.
type SubPoolService struct {
	repo       SubPoolRepository
	groupRepo  GroupRepository
	apiKeyRepo APIKeyRepository
	usageRepo  SubPoolUsageRepository
	membership *SubPoolMembership
	// authCache is optional; without it a pool change only takes effect once the
	// cached auth snapshot (which carries sub_pool_id) expires.
	authCache APIKeyAuthCacheInvalidator
}

// SetAuthCacheInvalidator attaches the auth snapshot invalidator. It is set
// after construction to avoid a dependency cycle with APIKeyService.
func (s *SubPoolService) SetAuthCacheInvalidator(invalidator APIKeyAuthCacheInvalidator) {
	if s == nil {
		return
	}
	s.authCache = invalidator
}

func (s *SubPoolService) invalidateAuthByKey(ctx context.Context, key string) {
	if s == nil || s.authCache == nil || key == "" {
		return
	}
	s.authCache.InvalidateAuthCacheByKey(ctx, key)
}

func (s *SubPoolService) invalidateAuthByGroup(ctx context.Context, groupID int64) {
	if s == nil || s.authCache == nil || groupID <= 0 {
		return
	}
	s.authCache.InvalidateAuthCacheByGroupID(ctx, groupID)
}

func NewSubPoolService(
	repo SubPoolRepository,
	groupRepo GroupRepository,
	apiKeyRepo APIKeyRepository,
	usageRepo SubPoolUsageRepository,
	membership *SubPoolMembership,
) *SubPoolService {
	return &SubPoolService{
		repo:       repo,
		groupRepo:  groupRepo,
		apiKeyRepo: apiKeyRepo,
		usageRepo:  usageRepo,
		membership: membership,
	}
}

// ── Pool lifecycle ────────────────────────────────────────────────────────

func (s *SubPoolService) ListGroupKeys(ctx context.Context, groupID int64) ([]SubPoolGroupKey, error) {
	return s.repo.ListGroupKeys(ctx, groupID)
}

func (s *SubPoolService) GetGroupDefaultPool(ctx context.Context, groupID int64) (*int64, error) {
	return s.repo.GetGroupDefaultPool(ctx, groupID)
}

func (s *SubPoolService) SetGroupDefaultPool(ctx context.Context, groupID, subPoolID int64) error {
	if _, err := s.requirePoolInGroup(ctx, subPoolID, groupID); err != nil {
		return err
	}
	return s.repo.SetGroupDefaultPool(ctx, groupID, &subPoolID)
}

func (s *SubPoolService) ClearGroupDefaultPool(ctx context.Context, groupID int64) error {
	return s.repo.SetGroupDefaultPool(ctx, groupID, nil)
}

func (s *SubPoolService) ListUserDefaults(ctx context.Context, groupID int64) ([]UserSubPoolDefault, error) {
	return s.repo.ListUserDefaults(ctx, groupID)
}

// SetUserDefaultPool pins a user to a pool and moves every one of their keys
// in this group. New keys inherit the same pin through EnsureBinding.
func (s *SubPoolService) SetUserDefaultPool(ctx context.Context, userID, groupID, subPoolID int64, operator string, note *string) error {
	if _, err := s.requirePoolInGroup(ctx, subPoolID, groupID); err != nil {
		return err
	}
	if err := s.repo.SetUserDefaultPool(ctx, UserSubPoolDefault{
		UserID:    userID,
		GroupID:   groupID,
		SubPoolID: subPoolID,
		Operator:  operator,
		Note:      note,
	}); err != nil {
		return err
	}
	keyIDs, err := s.repo.ListKeyIDsByUserGroup(ctx, userID, groupID)
	if err != nil {
		return err
	}
	for _, keyID := range keyIDs {
		if err := s.repo.BindKey(ctx, SubPoolBindInput{
			APIKeyID:  keyID,
			SubPoolID: subPoolID,
			GroupID:   groupID,
			Reason:    domain.SubPoolBindReasonUserDefault,
			Operator:  operator,
			Note:      note,
		}); err != nil {
			return err
		}
	}
	s.invalidateAuthByGroup(ctx, groupID)
	return nil
}

func (s *SubPoolService) ClearUserDefaultPool(ctx context.Context, userID, groupID int64) error {
	return s.repo.ClearUserDefaultPool(ctx, userID, groupID)
}

func (s *SubPoolService) requirePoolInGroup(ctx context.Context, subPoolID, groupID int64) (*SubPool, error) {
	pool, err := s.repo.GetByID(ctx, subPoolID)
	if err != nil {
		return nil, err
	}
	if pool.GroupID != groupID {
		return nil, ErrSubPoolGroupMismatch
	}
	return pool, nil
}

func (s *SubPoolService) ListByGroup(ctx context.Context, groupID int64) ([]SubPool, error) {
	return s.repo.ListByGroup(ctx, groupID)
}

func (s *SubPoolService) GetByID(ctx context.Context, id int64) (*SubPool, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *SubPoolService) Create(ctx context.Context, pool *SubPool) error {
	if err := s.normalize(pool); err != nil {
		return err
	}
	if _, err := s.groupRepo.GetByIDLite(ctx, pool.GroupID); err != nil {
		return err
	}
	exists, err := s.repo.ExistsByName(ctx, pool.GroupID, pool.Name, 0)
	if err != nil {
		return err
	}
	if exists {
		return ErrSubPoolExists
	}
	return s.repo.Create(ctx, pool)
}

func (s *SubPoolService) Update(ctx context.Context, pool *SubPool) error {
	if err := s.normalize(pool); err != nil {
		return err
	}
	current, err := s.repo.GetByID(ctx, pool.ID)
	if err != nil {
		return err
	}
	pool.GroupID = current.GroupID
	exists, err := s.repo.ExistsByName(ctx, pool.GroupID, pool.Name, pool.ID)
	if err != nil {
		return err
	}
	if exists {
		return ErrSubPoolExists
	}
	return s.repo.Update(ctx, pool)
}

func (s *SubPoolService) Delete(ctx context.Context, id int64) error {
	pool, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.membership.Invalidate(id)
	s.invalidateAuthByGroup(ctx, pool.GroupID)
	return nil
}

// SetAccounts replaces the pool's upstream accounts and refreshes the
// scheduling cache immediately: a stale allowlist here means traffic keeps
// hitting an account that was just pulled out of the pool.
func (s *SubPoolService) SetAccounts(ctx context.Context, subPoolID int64, accountIDs []int64) error {
	if err := s.repo.SetAccounts(ctx, subPoolID, dedupeInt64(accountIDs)); err != nil {
		return err
	}
	s.membership.Invalidate(subPoolID)
	return nil
}

func (s *SubPoolService) normalize(pool *SubPool) error {
	if pool == nil {
		return errors.New("sub-pool is nil")
	}
	pool.Name = strings.TrimSpace(pool.Name)
	if pool.Name == "" {
		return infraerrors.BadRequest("SUB_POOL_NAME_REQUIRED", "sub-pool name is required")
	}
	switch pool.Kind {
	case domain.SubPoolKindFormal, domain.SubPoolKindProbe:
	case "":
		pool.Kind = domain.SubPoolKindFormal
	default:
		return infraerrors.BadRequest("SUB_POOL_KIND_INVALID", "sub-pool kind must be formal or probe")
	}
	switch pool.Status {
	case domain.SubPoolStatusHealthy, domain.SubPoolStatusCooling, domain.SubPoolStatusClosed:
	case "":
		pool.Status = domain.SubPoolStatusHealthy
	default:
		return infraerrors.BadRequest("SUB_POOL_STATUS_INVALID", "sub-pool status must be healthy, cooling or closed")
	}
	if pool.KeySoftLimit < 0 {
		pool.KeySoftLimit = 0
	}
	return nil
}

// ── Key placement ─────────────────────────────────────────────────────────

// BindKey moves a key into a specific pool. The pool must live in the key's own
// group, otherwise the key would be billed against one group while consuming
// another group's accounts.
func (s *SubPoolService) BindKey(ctx context.Context, apiKeyID, subPoolID int64, operator string, note *string) error {
	apiKey, err := s.apiKeyRepo.GetByID(ctx, apiKeyID)
	if err != nil {
		return err
	}
	pool, err := s.repo.GetByID(ctx, subPoolID)
	if err != nil {
		return err
	}
	if apiKey.GroupID == nil || *apiKey.GroupID != pool.GroupID {
		return ErrSubPoolGroupMismatch
	}
	if err := s.repo.BindKey(ctx, SubPoolBindInput{
		APIKeyID:  apiKeyID,
		SubPoolID: subPoolID,
		GroupID:   pool.GroupID,
		Reason:    domain.SubPoolBindReasonAdminManual,
		Operator:  operator,
		Note:      note,
	}); err != nil {
		return err
	}
	s.invalidateAuthByKey(ctx, apiKey.Key)
	return nil
}

// EnsureBinding places a key that has no pool yet. It is called after key
// creation and after an admin moves a key between groups.
//
// Placement is user pin, then group default, then sort_order among healthy
// pools (probe first). A pinned high-risk user keeps landing on disposable
// accounts; everyone else can inherit the formal pool without a per-key click.
func (s *SubPoolService) EnsureBinding(ctx context.Context, apiKeyID int64) error {
	apiKey, err := s.apiKeyRepo.GetByID(ctx, apiKeyID)
	if err != nil {
		return err
	}
	if apiKey.GroupID == nil || *apiKey.GroupID <= 0 {
		return nil
	}
	group, err := s.groupRepo.GetByIDLite(ctx, *apiKey.GroupID)
	if err != nil {
		return err
	}
	if !group.SubPoolEnabled {
		return nil
	}

	pools, err := s.repo.ListByGroup(ctx, group.ID)
	if err != nil {
		return err
	}
	// A binding that already points inside this group is left alone: moving a
	// key between pools would rewrite its attribution history for no reason.
	if apiKey.SubPoolID != nil {
		for i := range pools {
			if pools[i].ID == *apiKey.SubPoolID {
				return nil
			}
		}
	}
	userDefault, err := s.repo.GetUserDefaultPool(ctx, apiKey.UserID, group.ID)
	if err != nil {
		return err
	}
	groupDefault, err := s.repo.GetGroupDefaultPool(ctx, group.ID)
	if err != nil {
		return err
	}
	target := resolvePoolForNewKey(pools, userDefault, groupDefault)
	if target == nil {
		return ErrSubPoolNoCapacity
	}
	if err := s.repo.BindKey(ctx, SubPoolBindInput{
		APIKeyID:  apiKeyID,
		SubPoolID: target.ID,
		GroupID:   group.ID,
		Reason:    domain.SubPoolBindReasonInitial,
		Operator:  domain.SubPoolBindOperatorSystem,
	}); err != nil {
		return err
	}
	s.invalidateAuthByKey(ctx, apiKey.Key)
	return nil
}

func findPoolByID(pools []SubPool, id int64) *SubPool {
	for i := range pools {
		if pools[i].ID == id {
			return &pools[i]
		}
	}
	return nil
}

// resolvePoolForNewKey is the inheritance order: user pin, then group default,
// then pickPoolForNewKey. A pin still requires the pool to exist and be
// healthy; the soft key cap is ignored so an admin placement cannot be bounced
// by a counter.
func resolvePoolForNewKey(pools []SubPool, userDefault, groupDefault *int64) *SubPool {
	for _, pinned := range []*int64{userDefault, groupDefault} {
		if pinned == nil || *pinned <= 0 {
			continue
		}
		if pool := findPoolByID(pools, *pinned); pool != nil && pool.Status == domain.SubPoolStatusHealthy {
			return pool
		}
	}
	return pickPoolForNewKey(pools)
}

// pickPoolForNewKey prefers a probe pool, then the lowest sort_order healthy
// pool. Manual isolation pools (观察池) should sit at a higher sort_order so
// ordinary new keys keep landing in the main formal pool even when the
// isolation pool is emptier and has no people cap.
//
// Only pools that still have capacity and at least one upstream account
// qualify: binding into an account-less pool would hand the user a key that
// cannot make a single call.
func pickPoolForNewKey(pools []SubPool) *SubPool {
	candidates := make([]*SubPool, 0, len(pools))
	for i := range pools {
		pool := &pools[i]
		if !pool.AcceptsNewBindings() || len(pool.AccountIDs) == 0 {
			continue
		}
		candidates = append(candidates, pool)
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].IsProbe() != candidates[j].IsProbe() {
			return candidates[i].IsProbe()
		}
		if candidates[i].SortOrder != candidates[j].SortOrder {
			return candidates[i].SortOrder < candidates[j].SortOrder
		}
		return candidates[i].BoundKeys < candidates[j].BoundKeys
	})
	return candidates[0]
}

// MigrateCleanKeys drains a burned pool: every key except the suspected ones is
// moved to another pool in the same group, and the source pool is put into
// cooling so it stops accepting new keys.
//
// Suspects stay behind on purpose. Moving them would spread the damage to a
// fresh set of accounts, and the whole point of the pool is that the evidence
// stays attached to the small group of keys that could have caused it.
func (s *SubPoolService) MigrateCleanKeys(ctx context.Context, subPoolID int64, suspectKeyIDs []int64, operator, reason string) (int, error) {
	pool, err := s.repo.GetByID(ctx, subPoolID)
	if err != nil {
		return 0, err
	}
	keyIDs, err := s.repo.ListKeyIDs(ctx, subPoolID)
	if err != nil {
		return 0, err
	}
	suspects := make(map[int64]struct{}, len(suspectKeyIDs))
	for _, id := range suspectKeyIDs {
		suspects[id] = struct{}{}
	}

	pools, err := s.repo.ListByGroup(ctx, pool.GroupID)
	if err != nil {
		return 0, err
	}

	moved := 0
	for _, keyID := range keyIDs {
		if _, suspect := suspects[keyID]; suspect {
			continue
		}
		target := pickMigrationTarget(pools, subPoolID)
		if target == nil {
			return moved, ErrSubPoolNoCapacity
		}
		if err := s.repo.BindKey(ctx, SubPoolBindInput{
			APIKeyID:  keyID,
			SubPoolID: target.ID,
			GroupID:   pool.GroupID,
			Reason:    domain.SubPoolBindReasonCoolingMigration,
			Operator:  operator,
			Note:      &reason,
		}); err != nil {
			return moved, err
		}
		target.BoundKeys++
		moved++
	}

	if pool.Status != domain.SubPoolStatusCooling {
		pool.Status = domain.SubPoolStatusCooling
		pool.CoolingReason = &reason
		if err := s.repo.Update(ctx, pool); err != nil {
			return moved, err
		}
	}
	if moved > 0 {
		s.invalidateAuthByGroup(ctx, pool.GroupID)
	}
	slog.Info("sub_pool_clean_keys_migrated",
		"sub_pool_id", subPoolID, "moved", moved, "suspects", len(suspects), "operator", operator)
	return moved, nil
}

// pickMigrationTarget mirrors pickPoolForNewKey but prefers a formal pool: a
// key that was already vouched for should not be demoted into the probe pool
// just because its neighbours got the pool burned.
func pickMigrationTarget(pools []SubPool, excludeID int64) *SubPool {
	var probe *SubPool
	var best *SubPool
	for i := range pools {
		pool := &pools[i]
		if pool.ID == excludeID || !pool.AcceptsNewBindings() || len(pool.AccountIDs) == 0 {
			continue
		}
		if pool.IsProbe() {
			if probe == nil || pool.BoundKeys < probe.BoundKeys {
				probe = pool
			}
			continue
		}
		if best == nil || pool.BoundKeys < best.BoundKeys {
			best = pool
		}
	}
	if best != nil {
		return best
	}
	return probe
}

// ── Sanctions ─────────────────────────────────────────────────────────────

// DemoteToProbe pushes a key back into the group's probe pool.
//
// This is the middle rung of the escalation ladder between rate limiting and a
// ban: the key keeps working, but on disposable accounts, and it has to serve
// probation again before it can return to the formal pools.
func (s *SubPoolService) DemoteToProbe(ctx context.Context, apiKeyID int64, operator string, note *string) error {
	apiKey, err := s.apiKeyRepo.GetByID(ctx, apiKeyID)
	if err != nil {
		return err
	}
	if apiKey.GroupID == nil || *apiKey.GroupID <= 0 {
		return ErrSubPoolNotEnabled
	}
	pools, err := s.repo.ListByGroup(ctx, *apiKey.GroupID)
	if err != nil {
		return err
	}
	target := pickProbePool(pools)
	if target == nil {
		return ErrSubPoolNoCapacity
	}
	if err := s.repo.BindKey(ctx, SubPoolBindInput{
		APIKeyID:  apiKeyID,
		SubPoolID: target.ID,
		GroupID:   *apiKey.GroupID,
		Reason:    domain.SubPoolBindReasonPunishDemotion,
		Operator:  operator,
		Note:      note,
	}); err != nil {
		return err
	}
	s.invalidateAuthByKey(ctx, apiKey.Key)
	slog.Info("sub_pool_key_demoted_to_probe",
		"api_key_id", apiKeyID, "sub_pool_id", target.ID, "operator", operator)
	return nil
}

// pickProbePool returns the emptiest probe pool that can actually serve traffic.
// Unlike graduation targets, a full probe pool is still acceptable: holding a
// sanctioned key is more important than respecting the soft cap.
func pickProbePool(pools []SubPool) *SubPool {
	var best *SubPool
	for i := range pools {
		pool := &pools[i]
		if !pool.IsProbe() || pool.Status != domain.SubPoolStatusHealthy || len(pool.AccountIDs) == 0 {
			continue
		}
		if best == nil || pool.BoundKeys < best.BoundKeys {
			best = pool
		}
	}
	return best
}

// DisableKey is the last rung: the key stops authenticating entirely. The pool
// binding is left intact so the incident history stays readable.
func (s *SubPoolService) DisableKey(ctx context.Context, apiKeyID int64, operator string) error {
	apiKey, err := s.apiKeyRepo.GetByID(ctx, apiKeyID)
	if err != nil {
		return err
	}
	if apiKey.Status == StatusAPIKeyDisabled {
		return nil
	}
	apiKey.Status = StatusAPIKeyDisabled
	if err := s.apiKeyRepo.Update(ctx, apiKey, APIKeyUpdateFields{Status: true}); err != nil {
		return err
	}
	s.invalidateAuthByKey(ctx, apiKey.Key)
	slog.Info("sub_pool_key_disabled", "api_key_id", apiKeyID, "operator", operator)
	return nil
}

// ── User-facing board ─────────────────────────────────────────────────────

// PeerBoard returns the anonymised view of who else shares the caller's pool.
// Ownership is enforced here rather than in the handler: this endpoint exposes
// other people's call patterns, so the check belongs next to the data.
func (s *SubPoolService) PeerBoard(ctx context.Context, userID, apiKeyID int64) (*SubPoolPeerBoard, error) {
	apiKey, err := s.apiKeyRepo.GetByID(ctx, apiKeyID)
	if err != nil {
		return nil, err
	}
	if apiKey.UserID != userID {
		return nil, ErrAPIKeyNotFound
	}
	if apiKey.SubPoolID == nil || *apiKey.SubPoolID <= 0 {
		return nil, ErrSubPoolNotEnabled
	}
	keyIDs, err := s.repo.ListKeyIDs(ctx, *apiKey.SubPoolID)
	if err != nil {
		return nil, err
	}

	windows := make([]SubPoolPeerWindow, 0, len(keyIDs))
	for _, keyID := range keyIDs {
		since := time.Time{}
		binding, err := s.repo.GetOpenBinding(ctx, keyID)
		if err != nil {
			return nil, err
		}
		if binding != nil {
			since = binding.BoundAt
		}
		windows = append(windows, SubPoolPeerWindow{APIKeyID: keyID, Since: since})
	}

	todayStart := timezone.Today()
	weekStart := timezone.Now().Add(-subPoolPeerWindow)
	activity, err := s.usageRepo.PeerActivity(ctx, windows, todayStart, weekStart)
	if err != nil {
		return nil, err
	}

	board := &SubPoolPeerBoard{
		MemberCount:     len(keyIDs),
		Peers:           make([]SubPoolPeer, 0, len(keyIDs)),
		WeekWindowHours: int(subPoolPeerWindow / time.Hour),
	}
	for i, keyID := range keyIDs {
		stat := activity[keyID]
		board.Peers = append(board.Peers, SubPoolPeer{
			Label:       peerLabel(i),
			IsSelf:      keyID == apiKeyID,
			FirstCallAt: stat.FirstCallAt,
			LastCallAt:  stat.LastCallAt,
			TodayCalls:  stat.TodayCalls,
			WeekCalls:   stat.WeekCalls,
		})
	}
	return board, nil
}

// subPoolPeerWindow is the rolling 7×24h window behind the "last 7 days" column.
const subPoolPeerWindow = 7 * 24 * time.Hour

// peerLabel derives A, B, ... Z, AA, AB ... from the key's position in the pool.
func peerLabel(index int) string {
	if index < 0 {
		return "?"
	}
	label := ""
	for {
		label = string(rune('A'+index%26)) + label
		index = index/26 - 1
		if index < 0 {
			return label
		}
	}
}

// TopKeysByAccount is the admin-side attribution report for a burned account.
func (s *SubPoolService) TopKeysByAccount(ctx context.Context, accountID int64, start, end time.Time, limit int) ([]SubPoolAccountKeyUsage, error) {
	if !end.After(start) {
		return nil, fmt.Errorf("end must be after start")
	}
	return s.usageRepo.TopKeysByAccount(ctx, accountID, start, end, limit)
}

func dedupeInt64(in []int64) []int64 {
	seen := make(map[int64]struct{}, len(in))
	out := make([]int64, 0, len(in))
	for _, v := range in {
		if v <= 0 {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
