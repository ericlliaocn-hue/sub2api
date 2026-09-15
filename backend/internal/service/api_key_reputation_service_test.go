package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubReputationRepo struct {
	penalties    []ReputationPenalty
	penaltyErr   error
	rows         map[int64]*APIKeyReputation
	sanctioned   map[int64]string
	resetKeepIDs []int64
	resetCalls   int
}

func newStubReputationRepo(penalties ...ReputationPenalty) *stubReputationRepo {
	return &stubReputationRepo{
		penalties:  penalties,
		rows:       map[int64]*APIKeyReputation{},
		sanctioned: map[int64]string{},
	}
}

func (r *stubReputationRepo) AggregatePenalties(context.Context, time.Time, time.Duration) ([]ReputationPenalty, error) {
	if r.penaltyErr != nil {
		return nil, r.penaltyErr
	}
	return r.penalties, nil
}

func (r *stubReputationRepo) Upsert(_ context.Context, rep *APIKeyReputation) error {
	existing := r.rows[rep.APIKeyID]
	stored := *rep
	if existing != nil {
		stored.Sanction = existing.Sanction
	} else if stored.Sanction == "" {
		stored.Sanction = ReputationSanctionNone
	}
	r.rows[rep.APIKeyID] = &stored
	return nil
}

func (r *stubReputationRepo) Get(_ context.Context, apiKeyID int64) (*APIKeyReputation, error) {
	return r.rows[apiKeyID], nil
}

func (r *stubReputationRepo) ListWorst(context.Context, int) ([]APIKeyReputation, error) {
	return nil, nil
}

func (r *stubReputationRepo) MarkSanctioned(_ context.Context, apiKeyID int64, sanction, reason string) error {
	r.sanctioned[apiKeyID] = sanction
	if row := r.rows[apiKeyID]; row != nil {
		row.Sanction = sanction
		row.SanctionReason = &reason
	}
	return nil
}

func (r *stubReputationRepo) ClearSanction(_ context.Context, apiKeyID int64) error {
	delete(r.sanctioned, apiKeyID)
	if row := r.rows[apiKeyID]; row != nil {
		row.Sanction = ReputationSanctionNone
	}
	return nil
}

func (r *stubReputationRepo) ResetScoresNotIn(_ context.Context, keep []int64) error {
	r.resetCalls++
	r.resetKeepIDs = append([]int64(nil), keep...)
	return nil
}

type stubReputationSettings struct{ policy ReputationPolicy }

func (s stubReputationSettings) GetReputationPolicy(context.Context) ReputationPolicy {
	return s.policy
}

type stubSanctioner struct {
	demoted   []int64
	disabled  []int64
	demoteErr error
}

func (s *stubSanctioner) DemoteToProbe(_ context.Context, apiKeyID int64, _ string, _ *string) error {
	if s.demoteErr != nil {
		return s.demoteErr
	}
	s.demoted = append(s.demoted, apiKeyID)
	return nil
}

func (s *stubSanctioner) DisableKey(_ context.Context, apiKeyID int64, _ string) error {
	s.disabled = append(s.disabled, apiKeyID)
	return nil
}

func enabledReputationPolicy() ReputationPolicy {
	return ReputationPolicy{Enabled: true, WindowDays: 30, DemoteBelow: 60, BanBelow: 25}
}

func newReputationServiceForTest(
	repo APIKeyReputationRepository,
	policy ReputationPolicy,
	sanctioner reputationSanctioner,
) *APIKeyReputationService {
	return NewAPIKeyReputationService(repo, stubReputationSettings{policy: policy}, sanctioner, nil)
}

func TestReputationSweepIsInertWhenDisabled(t *testing.T) {
	repo := newStubReputationRepo(ReputationPenalty{APIKeyID: 1, Penalty: 90})
	sanctioner := &stubSanctioner{}
	policy := enabledReputationPolicy()
	policy.Enabled = false

	result := newReputationServiceForTest(repo, policy, sanctioner).RunOnce(context.Background())

	if result.Scored != 0 || result.Demoted != 0 || result.Disabled != 0 {
		t.Fatalf("disabled policy should do nothing, got %+v", result)
	}
	if len(repo.rows) != 0 {
		t.Fatalf("disabled policy should not write scores")
	}
}

func TestReputationDemotesKeyBelowDemoteThreshold(t *testing.T) {
	// Penalty 50 -> score 50, which is under demote (60) but above ban (25).
	repo := newStubReputationRepo(ReputationPenalty{APIKeyID: 7, Penalty: 50, TotalHits: 4, SevereHits: 2})
	sanctioner := &stubSanctioner{}

	result := newReputationServiceForTest(repo, enabledReputationPolicy(), sanctioner).RunOnce(context.Background())

	if result.Demoted != 1 || result.Disabled != 0 {
		t.Fatalf("expected exactly one demotion, got %+v", result)
	}
	if len(sanctioner.demoted) != 1 || sanctioner.demoted[0] != 7 {
		t.Fatalf("expected key 7 demoted, got %v", sanctioner.demoted)
	}
	if repo.rows[7].Score != 50 {
		t.Fatalf("expected score 50, got %d", repo.rows[7].Score)
	}
	if repo.sanctioned[7] != ReputationSanctionDemoted {
		t.Fatalf("expected sanction recorded as demoted, got %q", repo.sanctioned[7])
	}
}

func TestReputationBansKeyBelowBanThreshold(t *testing.T) {
	repo := newStubReputationRepo(ReputationPenalty{APIKeyID: 9, Penalty: 80})
	sanctioner := &stubSanctioner{}

	result := newReputationServiceForTest(repo, enabledReputationPolicy(), sanctioner).RunOnce(context.Background())

	if result.Disabled != 1 || result.Demoted != 0 {
		t.Fatalf("expected exactly one ban, got %+v", result)
	}
	if len(sanctioner.demoted) != 0 {
		t.Fatalf("a bannable key should skip demotion, got %v", sanctioner.demoted)
	}
	if len(sanctioner.disabled) != 1 || sanctioner.disabled[0] != 9 {
		t.Fatalf("expected key 9 disabled, got %v", sanctioner.disabled)
	}
}

func TestReputationLeavesCleanKeyAlone(t *testing.T) {
	repo := newStubReputationRepo(ReputationPenalty{APIKeyID: 3, Penalty: 8, TotalHits: 1})
	sanctioner := &stubSanctioner{}

	result := newReputationServiceForTest(repo, enabledReputationPolicy(), sanctioner).RunOnce(context.Background())

	if result.Scored != 1 {
		t.Fatalf("expected the key to be scored, got %+v", result)
	}
	if result.Demoted != 0 || result.Disabled != 0 {
		t.Fatalf("a score of 92 must not be sanctioned, got %+v", result)
	}
}

func TestReputationDoesNotResanctionDemotedKey(t *testing.T) {
	repo := newStubReputationRepo(ReputationPenalty{APIKeyID: 4, Penalty: 50})
	repo.rows[4] = &APIKeyReputation{APIKeyID: 4, Score: 50, Sanction: ReputationSanctionDemoted}
	sanctioner := &stubSanctioner{}

	result := newReputationServiceForTest(repo, enabledReputationPolicy(), sanctioner).RunOnce(context.Background())

	if result.Demoted != 0 {
		t.Fatalf("an already-demoted key must not be demoted again, got %+v", result)
	}
	if len(sanctioner.demoted) != 0 {
		t.Fatalf("sanctioner should not have been called, got %v", sanctioner.demoted)
	}
}

func TestReputationEscalatesDemotedKeyToBan(t *testing.T) {
	// Already demoted, and the score has since fallen past the ban line.
	repo := newStubReputationRepo(ReputationPenalty{APIKeyID: 5, Penalty: 85})
	repo.rows[5] = &APIKeyReputation{APIKeyID: 5, Score: 50, Sanction: ReputationSanctionDemoted}
	sanctioner := &stubSanctioner{}

	result := newReputationServiceForTest(repo, enabledReputationPolicy(), sanctioner).RunOnce(context.Background())

	if result.Disabled != 1 {
		t.Fatalf("expected escalation to a ban, got %+v", result)
	}
	if repo.sanctioned[5] != ReputationSanctionDisabled {
		t.Fatalf("expected sanction recorded as disabled, got %q", repo.sanctioned[5])
	}
}

func TestReputationSkipsAlreadyDisabledKey(t *testing.T) {
	repo := newStubReputationRepo(ReputationPenalty{APIKeyID: 6, Penalty: 95})
	repo.rows[6] = &APIKeyReputation{APIKeyID: 6, Score: 5, Sanction: ReputationSanctionDisabled}
	sanctioner := &stubSanctioner{}

	result := newReputationServiceForTest(repo, enabledReputationPolicy(), sanctioner).RunOnce(context.Background())

	if result.Disabled != 0 {
		t.Fatalf("a disabled key must not be re-banned, got %+v", result)
	}
	if len(sanctioner.disabled) != 0 {
		t.Fatalf("sanctioner should not have been called, got %v", sanctioner.disabled)
	}
}

func TestReputationRecordsScoreWhenDemotionHasNowhereToGo(t *testing.T) {
	// A group with no probe pool cannot absorb a demotion. The score must still
	// be persisted so the key can be banned if it keeps sliding.
	repo := newStubReputationRepo(ReputationPenalty{APIKeyID: 8, Penalty: 50})
	sanctioner := &stubSanctioner{demoteErr: errors.New("no probe pool")}

	result := newReputationServiceForTest(repo, enabledReputationPolicy(), sanctioner).RunOnce(context.Background())

	if result.Demoted != 0 {
		t.Fatalf("a failed demotion must not be counted, got %+v", result)
	}
	if repo.rows[8] == nil || repo.rows[8].Score != 50 {
		t.Fatalf("score should still be recorded, got %+v", repo.rows[8])
	}
	if _, marked := repo.sanctioned[8]; marked {
		t.Fatalf("a failed demotion must not be recorded as applied")
	}
}

func TestReputationResetsKeysThatAgedOutOfWindow(t *testing.T) {
	repo := newStubReputationRepo(ReputationPenalty{APIKeyID: 11, Penalty: 20})
	sanctioner := &stubSanctioner{}

	newReputationServiceForTest(repo, enabledReputationPolicy(), sanctioner).RunOnce(context.Background())

	if repo.resetCalls != 1 {
		t.Fatalf("expected one reset call, got %d", repo.resetCalls)
	}
	if len(repo.resetKeepIDs) != 1 || repo.resetKeepIDs[0] != 11 {
		t.Fatalf("only freshly scored keys should be spared, got %v", repo.resetKeepIDs)
	}
}

func TestReputationSkipsSweepWhenAggregateFails(t *testing.T) {
	repo := newStubReputationRepo()
	repo.penaltyErr = errors.New("db down")
	sanctioner := &stubSanctioner{}

	result := newReputationServiceForTest(repo, enabledReputationPolicy(), sanctioner).RunOnce(context.Background())

	if result.Scored != 0 {
		t.Fatalf("a failed aggregate must not score anything, got %+v", result)
	}
	// Critically: no reset either. Wiping every score because the read failed
	// would hand every sanctioned key a clean slate.
	if repo.resetCalls != 0 {
		t.Fatalf("a failed aggregate must not reset scores, got %d calls", repo.resetCalls)
	}
}

func TestReputationScoreClamps(t *testing.T) {
	cases := []struct {
		name    string
		penalty int
		want    int
	}{
		{"no hits", 0, 100},
		{"moderate", 40, 60},
		{"floors at zero", 250, 0},
		{"negative penalty cannot exceed max", -10, 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := reputationScore(tc.penalty); got != tc.want {
				t.Fatalf("reputationScore(%d) = %d, want %d", tc.penalty, got, tc.want)
			}
		})
	}
}

// memorySettingRepo is the smallest SettingRepository that exercises the policy
// round-trip.
type memorySettingRepo struct{ values map[string]string }

func newMemorySettingRepo() *memorySettingRepo {
	return &memorySettingRepo{values: map[string]string{}}
}

func (r *memorySettingRepo) Get(context.Context, string) (*Setting, error) { return nil, nil }

func (r *memorySettingRepo) GetValue(_ context.Context, key string) (string, error) {
	return r.values[key], nil
}

func (r *memorySettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *memorySettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if v, ok := r.values[key]; ok {
			out[key] = v
		}
	}
	return out, nil
}

func (r *memorySettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *memorySettingRepo) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}

func (r *memorySettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func TestReputationPolicyDefaultsToDisabled(t *testing.T) {
	svc := NewSettingService(newMemorySettingRepo(), nil)

	policy := svc.GetReputationPolicy(context.Background())

	if policy.Enabled {
		t.Fatal("reputation scoring must ship off")
	}
	if policy.WindowDays != defaultReputationWindowDays ||
		policy.DemoteBelow != defaultReputationDemoteBelow ||
		policy.BanBelow != defaultReputationBanBelow {
		t.Fatalf("unexpected defaults: %+v", policy)
	}
}

func TestReputationPolicyRejectsUnreachableDemoteThreshold(t *testing.T) {
	svc := NewSettingService(newMemorySettingRepo(), nil)

	// Ban at 70 with demote at 60 would mean every demotable key is already
	// bannable, so demotion could never fire.
	err := svc.SetReputationPolicy(context.Background(), ReputationPolicy{
		Enabled: true, WindowDays: 30, DemoteBelow: 60, BanBelow: 70,
	})

	if err == nil {
		t.Fatal("expected a ban threshold above the demote threshold to be rejected")
	}
	if svc.GetReputationPolicy(context.Background()).Enabled {
		t.Fatal("a rejected policy must not be persisted")
	}
}

func TestReputationPolicyRoundTrips(t *testing.T) {
	svc := NewSettingService(newMemorySettingRepo(), nil)
	want := ReputationPolicy{Enabled: true, WindowDays: 14, DemoteBelow: 70, BanBelow: 30}

	if err := svc.SetReputationPolicy(context.Background(), want); err != nil {
		t.Fatalf("SetReputationPolicy: %v", err)
	}

	if got := svc.GetReputationPolicy(context.Background()); got != want {
		t.Fatalf("policy round-trip mismatch: got %+v, want %+v", got, want)
	}
}
