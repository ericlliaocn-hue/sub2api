package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

type stubGraduationRepo struct {
	SubPoolRepository
	candidates []SubPoolProbationCandidate
	pools      map[int64][]SubPool
	binds      []SubPoolBindInput
	bindErr    error
}

func (s *stubGraduationRepo) ListProbationCandidates(_ context.Context, _ time.Time, _ int) ([]SubPoolProbationCandidate, error) {
	return s.candidates, nil
}

func (s *stubGraduationRepo) ListByGroup(_ context.Context, groupID int64) ([]SubPool, error) {
	return s.pools[groupID], nil
}

func (s *stubGraduationRepo) BindKey(_ context.Context, in SubPoolBindInput) error {
	if s.bindErr != nil {
		return s.bindErr
	}
	s.binds = append(s.binds, in)
	return nil
}

type stubGraduationUsage struct {
	SubPoolUsageRepository
	violations    map[int64]int64
	violationsErr error
	peakDaily     map[int64]int64
}

func (s *stubGraduationUsage) CountViolationsSince(_ context.Context, apiKeyID int64, _ time.Time) (int64, error) {
	if s.violationsErr != nil {
		return 0, s.violationsErr
	}
	return s.violations[apiKeyID], nil
}

func (s *stubGraduationUsage) MaxDailyCallsSince(_ context.Context, apiKeyID int64, _ time.Time) (int64, error) {
	return s.peakDaily[apiKeyID], nil
}

type stubGraduationSettings struct {
	policy SubPoolGraduationPolicy
}

func (s stubGraduationSettings) GetSubPoolGraduationPolicy(context.Context) SubPoolGraduationPolicy {
	return s.policy
}

func probePool(id int64) SubPool {
	return SubPool{
		ID: id, GroupID: 1, Kind: domain.SubPoolKindProbe,
		Status: domain.SubPoolStatusHealthy, AccountIDs: []int64{100},
	}
}

func formalPool(id int64, boundKeys, softLimit int) SubPool {
	return SubPool{
		ID: id, GroupID: 1, Kind: domain.SubPoolKindFormal,
		Status: domain.SubPoolStatusHealthy, AccountIDs: []int64{200 + id},
		BoundKeys: boundKeys, KeySoftLimit: softLimit,
	}
}

func candidate(apiKeyID int64) SubPoolProbationCandidate {
	return SubPoolProbationCandidate{
		APIKeyID: apiKeyID, GroupID: 1, SubPoolID: 9,
		BoundAt: time.Now().Add(-30 * 24 * time.Hour),
	}
}

func newGraduationService(repo *stubGraduationRepo, usage *stubGraduationUsage, policy SubPoolGraduationPolicy) *SubPoolGraduationService {
	return NewSubPoolGraduationService(repo, usage, stubGraduationSettings{policy: policy}, nil)
}

func enabledPolicy() SubPoolGraduationPolicy {
	return SubPoolGraduationPolicy{Enabled: true, ProbationDays: 7}
}

func TestGraduationMovesCleanKeyToEmptiestFormalPool(t *testing.T) {
	repo := &stubGraduationRepo{
		candidates: []SubPoolProbationCandidate{candidate(11)},
		pools:      map[int64][]SubPool{1: {probePool(9), formalPool(2, 5, 8), formalPool(3, 1, 8)}},
	}
	svc := newGraduationService(repo, &stubGraduationUsage{}, enabledPolicy())

	if got := svc.RunOnce(context.Background()); got != 1 {
		t.Fatalf("expected 1 graduation, got %d", got)
	}
	if len(repo.binds) != 1 {
		t.Fatalf("expected 1 bind, got %d", len(repo.binds))
	}
	if repo.binds[0].SubPoolID != 3 {
		t.Errorf("expected the emptiest formal pool (3), got %d", repo.binds[0].SubPoolID)
	}
	if repo.binds[0].Reason != domain.SubPoolBindReasonProbeGraduation {
		t.Errorf("expected probe graduation reason, got %q", repo.binds[0].Reason)
	}
	if repo.binds[0].Operator != domain.SubPoolBindOperatorSystem {
		t.Errorf("expected system operator, got %q", repo.binds[0].Operator)
	}
}

// The whole point of probation is that misbehaviour keeps a key on disposable
// accounts, so a single confirmed violation must block graduation.
func TestGraduationSkipsKeyWithViolations(t *testing.T) {
	repo := &stubGraduationRepo{
		candidates: []SubPoolProbationCandidate{candidate(11)},
		pools:      map[int64][]SubPool{1: {probePool(9), formalPool(2, 0, 8)}},
	}
	usage := &stubGraduationUsage{violations: map[int64]int64{11: 1}}
	svc := newGraduationService(repo, usage, enabledPolicy())

	if got := svc.RunOnce(context.Background()); got != 0 {
		t.Fatalf("expected no graduation, got %d", got)
	}
	if len(repo.binds) != 0 {
		t.Fatalf("expected no bind, got %d", len(repo.binds))
	}
}

// A key whose record cannot be read is not "clean", it is unknown. Promoting it
// onto production accounts would defeat the probe pool.
func TestGraduationSkipsKeyWhenViolationLookupFails(t *testing.T) {
	repo := &stubGraduationRepo{
		candidates: []SubPoolProbationCandidate{candidate(11)},
		pools:      map[int64][]SubPool{1: {probePool(9), formalPool(2, 0, 8)}},
	}
	usage := &stubGraduationUsage{violationsErr: errors.New("db down")}
	svc := newGraduationService(repo, usage, enabledPolicy())

	if got := svc.RunOnce(context.Background()); got != 0 {
		t.Fatalf("expected no graduation, got %d", got)
	}
}

func TestGraduationRespectsDailyCallCap(t *testing.T) {
	repo := &stubGraduationRepo{
		candidates: []SubPoolProbationCandidate{candidate(11), candidate(12)},
		pools:      map[int64][]SubPool{1: {probePool(9), formalPool(2, 0, 8)}},
	}
	usage := &stubGraduationUsage{peakDaily: map[int64]int64{11: 5000, 12: 10}}
	policy := enabledPolicy()
	policy.MaxDailyCalls = 1000
	svc := newGraduationService(repo, usage, policy)

	if got := svc.RunOnce(context.Background()); got != 1 {
		t.Fatalf("expected 1 graduation, got %d", got)
	}
	if repo.binds[0].APIKeyID != 12 {
		t.Errorf("expected the quiet key (12) to graduate, got %d", repo.binds[0].APIKeyID)
	}
}

// Overfilling the last good pool is exactly how one burned pool takes down the
// next one, so a full target means the key waits in probation.
func TestGraduationLeavesKeyWhenNoFormalPoolHasRoom(t *testing.T) {
	repo := &stubGraduationRepo{
		candidates: []SubPoolProbationCandidate{candidate(11)},
		pools:      map[int64][]SubPool{1: {probePool(9), formalPool(2, 8, 8)}},
	}
	svc := newGraduationService(repo, &stubGraduationUsage{}, enabledPolicy())

	if got := svc.RunOnce(context.Background()); got != 0 {
		t.Fatalf("expected no graduation, got %d", got)
	}
}

// Graduating into another probe pool would only restart the clock.
func TestGraduationNeverTargetsAnotherProbePool(t *testing.T) {
	repo := &stubGraduationRepo{
		candidates: []SubPoolProbationCandidate{candidate(11)},
		pools:      map[int64][]SubPool{1: {probePool(9), probePool(10)}},
	}
	svc := newGraduationService(repo, &stubGraduationUsage{}, enabledPolicy())

	if got := svc.RunOnce(context.Background()); got != 0 {
		t.Fatalf("expected no graduation, got %d", got)
	}
}

// A formal pool without accounts would hand the user a key that cannot make a
// single call.
func TestGraduationSkipsAccountlessFormalPool(t *testing.T) {
	empty := formalPool(2, 0, 8)
	empty.AccountIDs = nil
	repo := &stubGraduationRepo{
		candidates: []SubPoolProbationCandidate{candidate(11)},
		pools:      map[int64][]SubPool{1: {probePool(9), empty}},
	}
	svc := newGraduationService(repo, &stubGraduationUsage{}, enabledPolicy())

	if got := svc.RunOnce(context.Background()); got != 0 {
		t.Fatalf("expected no graduation, got %d", got)
	}
}

func TestGraduationIsInertWhenDisabled(t *testing.T) {
	repo := &stubGraduationRepo{
		candidates: []SubPoolProbationCandidate{candidate(11)},
		pools:      map[int64][]SubPool{1: {probePool(9), formalPool(2, 0, 8)}},
	}
	svc := newGraduationService(repo, &stubGraduationUsage{}, SubPoolGraduationPolicy{ProbationDays: 7})

	if got := svc.RunOnce(context.Background()); got != 0 {
		t.Fatalf("expected no graduation while disabled, got %d", got)
	}
	if len(repo.binds) != 0 {
		t.Fatalf("expected no bind while disabled, got %d", len(repo.binds))
	}
}

// Capacity must be tracked within a sweep, otherwise a batch of graduates all
// pick the same "emptiest" pool and blow straight through its soft limit.
func TestGraduationTracksCapacityWithinOneSweep(t *testing.T) {
	repo := &stubGraduationRepo{
		candidates: []SubPoolProbationCandidate{candidate(11), candidate(12), candidate(13)},
		pools:      map[int64][]SubPool{1: {probePool(9), formalPool(2, 0, 2)}},
	}
	svc := newGraduationService(repo, &stubGraduationUsage{}, enabledPolicy())

	if got := svc.RunOnce(context.Background()); got != 2 {
		t.Fatalf("expected 2 graduations before the soft limit, got %d", got)
	}
}

func TestParsePositiveSettingInt(t *testing.T) {
	cases := map[string]int{"7": 7, " 14 ": 14, "0": 0, "-3": 0, "": 0, "abc": 0}
	for in, want := range cases {
		if got := parsePositiveSettingInt(in); got != want {
			t.Errorf("parsePositiveSettingInt(%q) = %d, want %d", in, got, want)
		}
	}
}
