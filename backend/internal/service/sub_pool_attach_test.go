package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

func TestPickFormalAndObservationPools(t *testing.T) {
	formal := SubPool{ID: 6, Name: "正池", Kind: domain.SubPoolKindFormal, Status: domain.SubPoolStatusHealthy, SortOrder: 0}
	// Production historically stored 观察池 as kind=formal; name still wins.
	observation := SubPool{ID: 7, Name: "观察池", Kind: domain.SubPoolKindFormal, Status: domain.SubPoolStatusCooling, SortOrder: 1}
	spare := SubPool{ID: 5, Name: "备用", Kind: domain.SubPoolKindFormal, Status: domain.SubPoolStatusHealthy, SortOrder: 2}
	closed := SubPool{ID: 8, Name: "旧观察", Kind: domain.SubPoolKindProbe, Status: domain.SubPoolStatusClosed, SortOrder: 0}

	gotFormal := PickFormalPool([]SubPool{observation, spare, formal, closed})
	if gotFormal == nil || gotFormal.ID != 6 {
		t.Fatalf("formal = %+v, want 正池 6", gotFormal)
	}
	gotObs := PickObservationPool([]SubPool{observation, spare, formal, closed})
	if gotObs == nil || gotObs.ID != 7 {
		t.Fatalf("observation = %+v, want 观察池 7", gotObs)
	}
}

func TestNormalizeAttachSubPools(t *testing.T) {
	got, err := NormalizeAttachSubPools(" BOTH ")
	if err != nil || got != AttachSubPoolsBoth {
		t.Fatalf("both: got %q err %v", got, err)
	}
	got, err = NormalizeAttachSubPools("")
	if err != nil || got != "" {
		t.Fatalf("empty: got %q err %v", got, err)
	}
	if _, err := NormalizeAttachSubPools("all"); err == nil {
		t.Fatal("expected invalid mode error")
	}
}

type attachPoolRepoStub struct {
	SubPoolRepository
	pools    []SubPool
	setCalls [][]int64
	setIDs   []int64
}

func (s *attachPoolRepoStub) ListByGroup(context.Context, int64) ([]SubPool, error) {
	out := make([]SubPool, len(s.pools))
	copy(out, s.pools)
	return out, nil
}

func (s *attachPoolRepoStub) SetAccounts(_ context.Context, subPoolID int64, accountIDs []int64) error {
	s.setIDs = append(s.setIDs, subPoolID)
	copied := append([]int64(nil), accountIDs...)
	s.setCalls = append(s.setCalls, copied)
	return nil
}

func (s *attachPoolRepoStub) GetByID(_ context.Context, id int64) (*SubPool, error) {
	for i := range s.pools {
		if s.pools[i].ID == id {
			pool := s.pools[i]
			return &pool, nil
		}
	}
	return &SubPool{ID: id, Status: domain.SubPoolStatusHealthy}, nil
}

func (s *attachPoolRepoStub) Update(_ context.Context, pool *SubPool) error {
	if pool == nil {
		return nil
	}
	for i := range s.pools {
		if s.pools[i].ID == pool.ID {
			s.pools[i] = *pool
			return nil
		}
	}
	return nil
}

type attachGroupRepoStub struct {
	GroupRepository
	enabled bool
}

func (s *attachGroupRepoStub) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	return &Group{ID: id, SubPoolEnabled: s.enabled}, nil
}

func TestAttachAccountOnCreateBothPools(t *testing.T) {
	repo := &attachPoolRepoStub{pools: []SubPool{
		{ID: 6, Name: "正池", Kind: domain.SubPoolKindFormal, Status: domain.SubPoolStatusHealthy, AccountIDs: []int64{10}},
		{ID: 7, Name: "观察池", Kind: domain.SubPoolKindFormal, Status: domain.SubPoolStatusHealthy, AccountIDs: []int64{11}},
	}}
	svc := NewSubPoolService(repo, &attachGroupRepoStub{enabled: true}, nil, nil, nil)

	if err := svc.AttachAccountOnCreate(context.Background(), 99, []int64{19, 19}, AttachSubPoolsBoth); err != nil {
		t.Fatal(err)
	}
	if len(repo.setIDs) != 2 || repo.setIDs[0] != 6 || repo.setIDs[1] != 7 {
		t.Fatalf("set pool ids = %v", repo.setIDs)
	}
	if got := repo.setCalls[0]; len(got) != 2 || got[0] != 10 || got[1] != 99 {
		t.Fatalf("formal accounts = %v", got)
	}
	if got := repo.setCalls[1]; len(got) != 2 || got[0] != 11 || got[1] != 99 {
		t.Fatalf("observation accounts = %v", got)
	}
}

func TestSetAccountsUncoolsUnavailablePool(t *testing.T) {
	reason := domain.SubPoolCoolingReasonAccountsUnavailable
	future := time.Now().Add(15 * time.Minute)
	repo := &attachPoolRepoStub{pools: []SubPool{{
		ID: 6, Name: "正池", Kind: domain.SubPoolKindFormal,
		Status: domain.SubPoolStatusCooling, AccountIDs: []int64{10},
		CoolingReason: &reason, CoolingUntil: &future,
	}}}
	svc := NewSubPoolService(repo, &attachGroupRepoStub{enabled: true}, nil, nil, NewSubPoolMembership(repo))
	if err := svc.SetAccounts(context.Background(), 6, []int64{10, 99}); err != nil {
		t.Fatal(err)
	}
	if repo.pools[0].Status != domain.SubPoolStatusHealthy {
		t.Fatalf("status = %q, want healthy", repo.pools[0].Status)
	}
	if repo.pools[0].CoolingReason != nil || repo.pools[0].CoolingUntil != nil {
		t.Fatal("expected cooling fields cleared")
	}
}

func TestAttachAccountOnCreateSkipsDisabledGroup(t *testing.T) {
	repo := &attachPoolRepoStub{pools: []SubPool{
		{ID: 6, Name: "正池", Kind: domain.SubPoolKindFormal, Status: domain.SubPoolStatusHealthy},
	}}
	svc := NewSubPoolService(repo, &attachGroupRepoStub{enabled: false}, nil, nil, nil)
	if err := svc.AttachAccountOnCreate(context.Background(), 99, []int64{3}, AttachSubPoolsBoth); err != nil {
		t.Fatal(err)
	}
	if len(repo.setIDs) != 0 {
		t.Fatalf("expected no attach, got %v", repo.setIDs)
	}
}
