package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

type stubCoolingRepo struct {
	SubPoolRepository
	pools       []SubPool
	updated     []SubPool
	binds       []SubPoolBindInput
	keyIDs      map[int64][]int64
	autoRecent  map[int64]int
	openBinding map[int64]*SubPoolBinding
}

func (s *stubCoolingRepo) ListPoolsInEnabledGroups(context.Context) ([]SubPool, error) {
	return s.pools, nil
}

func (s *stubCoolingRepo) ListByGroup(_ context.Context, groupID int64) ([]SubPool, error) {
	out := make([]SubPool, 0, len(s.pools))
	for _, p := range s.pools {
		if p.GroupID == groupID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *stubCoolingRepo) GetByID(_ context.Context, id int64) (*SubPool, error) {
	for i := range s.pools {
		if s.pools[i].ID == id {
			pool := s.pools[i]
			return &pool, nil
		}
	}
	return nil, ErrSubPoolNotFound
}

func (s *stubCoolingRepo) Update(_ context.Context, pool *SubPool) error {
	s.updated = append(s.updated, *pool)
	return nil
}

func (s *stubCoolingRepo) ListKeyIDs(_ context.Context, subPoolID int64) ([]int64, error) {
	return s.keyIDs[subPoolID], nil
}

func (s *stubCoolingRepo) GetOpenBinding(_ context.Context, apiKeyID int64) (*SubPoolBinding, error) {
	return s.openBinding[apiKeyID], nil
}

func (s *stubCoolingRepo) CountAutoMigrationsSince(_ context.Context, apiKeyID int64, _ time.Time) (int, error) {
	return s.autoRecent[apiKeyID], nil
}

func (s *stubCoolingRepo) BindKey(_ context.Context, in SubPoolBindInput) error {
	s.binds = append(s.binds, in)
	return nil
}

type stubCoolingAccounts struct {
	schedulable map[int64]bool
}

func (s *stubCoolingAccounts) GetByIDs(_ context.Context, ids []int64) ([]*Account, error) {
	out := make([]*Account, 0, len(ids))
	for _, id := range ids {
		account := &Account{ID: id, Status: StatusActive, Schedulable: s.schedulable[id]}
		out = append(out, account)
	}
	return out, nil
}

type stubCoolingUsage struct {
	SubPoolUsageRepository
	usage      map[int64]SubPoolAccountKeyUsage
	violations map[int64]int64
}

func (s *stubCoolingUsage) KeyUsageInWindow(_ context.Context, ids []int64, _, _ time.Time) (map[int64]SubPoolAccountKeyUsage, error) {
	out := make(map[int64]SubPoolAccountKeyUsage, len(ids))
	for _, id := range ids {
		if stat, ok := s.usage[id]; ok {
			out[id] = stat
		}
	}
	return out, nil
}

func (s *stubCoolingUsage) CountViolationsSince(_ context.Context, apiKeyID int64, _ time.Time) (int64, error) {
	return s.violations[apiKeyID], nil
}

func newCoolingService(repo *stubCoolingRepo, accounts *stubCoolingAccounts, usage *stubCoolingUsage) *SubPoolCoolingService {
	subPools := NewSubPoolService(repo, nil, nil, usage, nil)
	return NewSubPoolCoolingService(repo, accounts, subPools, nil, nil)
}

func healthyFormal(id int64, accountIDs ...int64) SubPool {
	return SubPool{
		ID: id, GroupID: 1, Kind: domain.SubPoolKindFormal,
		Status: domain.SubPoolStatusHealthy, AccountIDs: accountIDs, KeySoftLimit: 8,
	}
}

func TestCoolingMarksPoolWithNoUsableAccount(t *testing.T) {
	repo := &stubCoolingRepo{pools: []SubPool{healthyFormal(1, 10, 11)}}
	accounts := &stubCoolingAccounts{schedulable: map[int64]bool{10: false, 11: false}}
	svc := newCoolingService(repo, accounts, &stubCoolingUsage{})

	result := svc.RunOnce(context.Background())

	if result.Cooled != 1 {
		t.Fatalf("expected 1 pool cooled, got %d", result.Cooled)
	}
	if len(repo.updated) != 1 || repo.updated[0].Status != domain.SubPoolStatusCooling {
		t.Fatalf("expected the pool to be set cooling, got %+v", repo.updated)
	}
	if repo.updated[0].CoolingReason == nil ||
		*repo.updated[0].CoolingReason != domain.SubPoolCoolingReasonAccountsUnavailable {
		t.Error("expected the automatic cooling reason to be recorded")
	}
}

// One live account is enough to keep serving. Cooling a pool that still works
// would cut off its users for nothing.
func TestCoolingLeavesPoolWithOneUsableAccount(t *testing.T) {
	repo := &stubCoolingRepo{pools: []SubPool{healthyFormal(1, 10, 11)}}
	accounts := &stubCoolingAccounts{schedulable: map[int64]bool{10: false, 11: true}}
	svc := newCoolingService(repo, accounts, &stubCoolingUsage{})

	if result := svc.RunOnce(context.Background()); result.Cooled != 0 {
		t.Fatalf("expected no cooling, got %d", result.Cooled)
	}
}

// A pool with no accounts is misconfigured, not burned. Cooling it would hide
// the real problem behind an incident state.
func TestCoolingIgnoresPoolWithoutAccounts(t *testing.T) {
	repo := &stubCoolingRepo{pools: []SubPool{healthyFormal(1)}}
	svc := newCoolingService(repo, &stubCoolingAccounts{}, &stubCoolingUsage{})

	if result := svc.RunOnce(context.Background()); result.Cooled != 0 {
		t.Fatalf("expected no cooling for an empty pool, got %d", result.Cooled)
	}
}

func TestCoolingDrainsCleanKeysButKeepsSuspects(t *testing.T) {
	repo := &stubCoolingRepo{
		pools:  []SubPool{healthyFormal(1, 10), healthyFormal(2, 20)},
		keyIDs: map[int64][]int64{1: {101, 102}},
	}
	accounts := &stubCoolingAccounts{schedulable: map[int64]bool{10: false, 20: true}}
	usage := &stubCoolingUsage{violations: map[int64]int64{101: 3}}
	svc := newCoolingService(repo, accounts, usage)

	result := svc.RunOnce(context.Background())

	if result.Migrated != 1 {
		t.Fatalf("expected 1 migration, got %d", result.Migrated)
	}
	if repo.binds[0].APIKeyID != 102 {
		t.Errorf("expected the clean key (102) to move, got %d", repo.binds[0].APIKeyID)
	}
	if repo.binds[0].SubPoolID != 2 {
		t.Errorf("expected the healthy pool (2) as target, got %d", repo.binds[0].SubPoolID)
	}
	if repo.binds[0].Reason != domain.SubPoolBindReasonCoolingMigration {
		t.Errorf("expected cooling_migration reason, got %q", repo.binds[0].Reason)
	}
}

// A pool that keeps flapping would otherwise walk one user through every pool
// in the group, resetting their probation and peer history each hop.
func TestCoolingDebouncesRecentlyMigratedKey(t *testing.T) {
	repo := &stubCoolingRepo{
		pools:      []SubPool{healthyFormal(1, 10), healthyFormal(2, 20)},
		keyIDs:     map[int64][]int64{1: {101}},
		autoRecent: map[int64]int{101: 1},
	}
	accounts := &stubCoolingAccounts{schedulable: map[int64]bool{10: false, 20: true}}
	svc := newCoolingService(repo, accounts, &stubCoolingUsage{})

	if result := svc.RunOnce(context.Background()); result.Migrated != 0 {
		t.Fatalf("expected the debounce to hold the key, got %d migrations", result.Migrated)
	}
}

func TestCoolingRecoversPoolAfterMinimumDuration(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	reason := domain.SubPoolCoolingReasonAccountsUnavailable
	repo := &stubCoolingRepo{pools: []SubPool{{
		ID: 1, GroupID: 1, Kind: domain.SubPoolKindFormal,
		Status: domain.SubPoolStatusCooling, AccountIDs: []int64{10},
		CoolingUntil: &past, CoolingReason: &reason,
	}}}
	accounts := &stubCoolingAccounts{schedulable: map[int64]bool{10: true}}
	svc := newCoolingService(repo, accounts, &stubCoolingUsage{})

	result := svc.RunOnce(context.Background())

	if result.Recovered != 1 {
		t.Fatalf("expected 1 recovery, got %d", result.Recovered)
	}
	if repo.updated[0].Status != domain.SubPoolStatusHealthy {
		t.Errorf("expected the pool back to healthy, got %q", repo.updated[0].Status)
	}
	if repo.updated[0].CoolingReason != nil {
		t.Error("expected the cooling reason to be cleared")
	}
}

type stubCoolingAccountsWriter struct {
	stubCoolingAccounts
	enabled []int64
}

func (s *stubCoolingAccountsWriter) GetByIDs(_ context.Context, ids []int64) ([]*Account, error) {
	out := make([]*Account, 0, len(ids))
	for _, id := range ids {
		account := &Account{
			ID:          id,
			Status:      StatusActive,
			Schedulable: s.schedulable[id],
			Type:        AccountTypeOAuth,
			Credentials: map[string]any{"plan_type": "pro"},
		}
		out = append(out, account)
	}
	return out, nil
}

func (s *stubCoolingAccountsWriter) SetSchedulable(_ context.Context, id int64, schedulable bool) error {
	if s.schedulable == nil {
		s.schedulable = map[int64]bool{}
	}
	s.schedulable[id] = schedulable
	s.enabled = append(s.enabled, id)
	return nil
}

func TestCoolingPromotesReserveInsteadOfEmptyingPool(t *testing.T) {
	repo := &stubCoolingRepo{pools: []SubPool{{
		ID: 1, GroupID: 19, Kind: domain.SubPoolKindFormal,
		Status: domain.SubPoolStatusHealthy, AccountIDs: []int64{10},
	}}}
	accounts := &stubCoolingAccountsWriter{stubCoolingAccounts: stubCoolingAccounts{schedulable: map[int64]bool{10: false}}}
	subPools := NewSubPoolService(repo, nil, nil, &stubCoolingUsage{}, nil)
	svc := NewSubPoolCoolingService(repo, accounts, subPools, nil, nil)

	result := svc.RunOnce(context.Background())
	if result.Cooled != 0 {
		t.Fatalf("expected reserve promote to skip cooling, got cooled=%d", result.Cooled)
	}
	if len(accounts.enabled) != 1 || accounts.enabled[0] != 10 {
		t.Fatalf("expected reserve 10 to be enabled, got %v", accounts.enabled)
	}
}

func TestCoolingRecoversImmediatelyWhenAccountIsUsable(t *testing.T) {
	future := time.Now().Add(10 * time.Minute)
	reason := domain.SubPoolCoolingReasonAccountsUnavailable
	repo := &stubCoolingRepo{pools: []SubPool{{
		ID: 1, GroupID: 1, Status: domain.SubPoolStatusCooling, AccountIDs: []int64{10},
		CoolingUntil: &future, CoolingReason: &reason,
	}}}
	accounts := &stubCoolingAccounts{schedulable: map[int64]bool{10: true}}
	svc := newCoolingService(repo, accounts, &stubCoolingUsage{})

	result := svc.RunOnce(context.Background())
	if result.Recovered != 1 {
		t.Fatalf("expected immediate recovery once a car is usable, got %d", result.Recovered)
	}
	if repo.updated[0].Status != domain.SubPoolStatusHealthy {
		t.Errorf("expected healthy, got %q", repo.updated[0].Status)
	}
}

// An operator who cooled a pool by hand decides when it comes back.
func TestCoolingDoesNotRecoverManuallyCooledPool(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	reason := "manual investigation"
	repo := &stubCoolingRepo{pools: []SubPool{{
		ID: 1, GroupID: 1, Status: domain.SubPoolStatusCooling, AccountIDs: []int64{10},
		CoolingUntil: &past, CoolingReason: &reason,
	}}}
	accounts := &stubCoolingAccounts{schedulable: map[int64]bool{10: true}}
	svc := newCoolingService(repo, accounts, &stubCoolingUsage{})

	if result := svc.RunOnce(context.Background()); result.Recovered != 0 {
		t.Fatalf("expected a manually cooled pool to stay cooling, got %d", result.Recovered)
	}
}

func TestClassifySuspect(t *testing.T) {
	cases := []struct {
		name    string
		row     SubPoolKeyAttribution
		total   int64
		suspect bool
		why     string
	}{
		{"violations win outright", SubPoolKeyAttribution{Violations: 1}, 0, true, "violations"},
		{"dominant share on a busy pool", SubPoolKeyAttribution{Calls: 80, Share: 0.8}, 100, true, "traffic_share"},
		{"same share on a quiet pool means nothing", SubPoolKeyAttribution{Calls: 3, Share: 0.75}, 4, false, ""},
		{"minority share is clean", SubPoolKeyAttribution{Calls: 10, Share: 0.1}, 100, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			suspect, why := classifySuspect(tc.row, tc.total)
			if suspect != tc.suspect || why != tc.why {
				t.Errorf("got (%v, %q), want (%v, %q)", suspect, why, tc.suspect, tc.why)
			}
		})
	}
}
