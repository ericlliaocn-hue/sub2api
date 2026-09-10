package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

type stubSubPoolRepo struct {
	SubPoolRepository
	accounts map[int64][]int64
	err      error
	calls    int
}

func (s *stubSubPoolRepo) ListAccountIDs(_ context.Context, subPoolID int64) ([]int64, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.accounts[subPoolID], nil
}

func ctxWithSubPool(id int64) context.Context {
	return context.WithValue(context.Background(), ctxkey.SubPoolID, id)
}

func accountsWithIDs(ids ...int64) []Account {
	out := make([]Account, 0, len(ids))
	for _, id := range ids {
		out = append(out, Account{ID: id})
	}
	return out
}

func accountIDs(accounts []Account) []int64 {
	out := make([]int64, 0, len(accounts))
	for _, acc := range accounts {
		out = append(out, acc.ID)
	}
	return out
}

func TestFilterAccountsBySubPoolKeepsOnlyPoolMembers(t *testing.T) {
	m := NewSubPoolMembership(&stubSubPoolRepo{accounts: map[int64][]int64{7: {2, 4}}})

	got := m.FilterAccountsBySubPool(ctxWithSubPool(7), accountsWithIDs(1, 2, 3, 4, 5))

	want := []int64{2, 4}
	if ids := accountIDs(got); len(ids) != len(want) || ids[0] != want[0] || ids[1] != want[1] {
		t.Fatalf("expected accounts %v, got %v", want, ids)
	}
}

// An empty pool must not spill over to the rest of the group: that would hand
// the key a fresh set of accounts to burn.
func TestFilterAccountsBySubPoolReturnsEmptyWhenPoolHasNoAccounts(t *testing.T) {
	m := NewSubPoolMembership(&stubSubPoolRepo{accounts: map[int64][]int64{7: {}}})

	got := m.FilterAccountsBySubPool(ctxWithSubPool(7), accountsWithIDs(1, 2, 3))

	if len(got) != 0 {
		t.Fatalf("expected no candidates, got %v", accountIDs(got))
	}
}

func TestFilterAccountsBySubPoolIsNoOpWithoutBinding(t *testing.T) {
	m := NewSubPoolMembership(&stubSubPoolRepo{accounts: map[int64][]int64{7: {2}}})

	got := m.FilterAccountsBySubPool(context.Background(), accountsWithIDs(1, 2, 3))

	if len(got) != 3 {
		t.Fatalf("expected all candidates, got %v", accountIDs(got))
	}
}

// A nil resolver means the feature is not wired; scheduling must behave exactly
// as it did before sub-pools existed.
func TestFilterAccountsBySubPoolIsNoOpWhenResolverMissing(t *testing.T) {
	var m *SubPoolMembership

	got := m.FilterAccountsBySubPool(ctxWithSubPool(7), accountsWithIDs(1, 2, 3))

	if len(got) != 3 {
		t.Fatalf("expected all candidates, got %v", accountIDs(got))
	}
}

// Losing the database must degrade to the previous behaviour rather than to a
// hard outage for every key in a sub-pool.
func TestFilterAccountsBySubPoolFallsBackWhenMembershipUnresolved(t *testing.T) {
	m := NewSubPoolMembership(&stubSubPoolRepo{err: errors.New("db down")})

	got := m.FilterAccountsBySubPool(ctxWithSubPool(7), accountsWithIDs(1, 2, 3))

	if len(got) != 3 {
		t.Fatalf("expected all candidates, got %v", accountIDs(got))
	}
}

func TestAllowedAccountIDsCachesLookups(t *testing.T) {
	repo := &stubSubPoolRepo{accounts: map[int64][]int64{7: {2, 4}}}
	m := NewSubPoolMembership(repo)
	ctx := context.Background()

	for range 3 {
		if _, ok := m.AllowedAccountIDs(ctx, 7); !ok {
			t.Fatal("expected membership to resolve")
		}
	}
	if repo.calls != 1 {
		t.Fatalf("expected 1 repository call, got %d", repo.calls)
	}

	m.Invalidate(7)
	if _, ok := m.AllowedAccountIDs(ctx, 7); !ok {
		t.Fatal("expected membership to resolve after invalidation")
	}
	if repo.calls != 2 {
		t.Fatalf("expected 2 repository calls after invalidation, got %d", repo.calls)
	}
}
