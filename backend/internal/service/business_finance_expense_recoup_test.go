package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExpenseRecoupWindowUsesPeriodAndAccount(t *testing.T) {
	occurred := time.Date(2026, 9, 20, 11, 26, 0, 0, time.UTC)
	start := time.Date(2026, 9, 20, 11, 26, 0, 0, time.UTC)
	end := time.Date(2026, 9, 20, 15, 0, 0, 0, time.UTC)
	now := time.Date(2026, 9, 20, 13, 45, 0, 0, time.UTC)

	item := ExpenseEntry{
		Amount:      45,
		OccurredAt:  occurred,
		PeriodStart: &start,
		PeriodEnd:   &end,
		Scope:       map[string]any{"account_id": float64(66274)},
	}

	accountID, gotStart, gotEnd, ok := ExpenseRecoupWindow(item, now)
	require.True(t, ok)
	require.Equal(t, int64(66274), accountID)
	require.True(t, start.Equal(gotStart))
	require.True(t, end.Equal(gotEnd))
}

func TestExpenseRecoupWindowOpenEndUsesNow(t *testing.T) {
	occurred := time.Date(2026, 9, 19, 10, 54, 0, 0, time.UTC)
	now := time.Date(2026, 9, 20, 13, 45, 0, 0, time.UTC)
	item := ExpenseEntry{
		OccurredAt: occurred,
		Scope:      map[string]any{"account_id": json.Number("66268")},
	}

	accountID, start, end, ok := ExpenseRecoupWindow(item, now)
	require.True(t, ok)
	require.Equal(t, int64(66268), accountID)
	require.True(t, occurred.Equal(start))
	require.True(t, now.Equal(end))
}

func TestExpenseRecoupWindowRequiresAccount(t *testing.T) {
	now := time.Now().UTC()
	_, _, _, ok := ExpenseRecoupWindow(ExpenseEntry{
		OccurredAt: now.Add(-time.Hour),
		Scope:      map[string]any{"group_id": float64(19)},
	}, now)
	require.False(t, ok)
}

func TestApplyExpenseRecoupUsesOneToOneRate(t *testing.T) {
	item := ExpenseEntry{Amount: 45, ExchangeRate: 1, Scope: map[string]any{"account_id": 66274}}
	ApplyExpenseRecoup(&item, 66274, 32.89, 3176, "lindaphillipsn905+inv@gmail.com")
	require.NotNil(t, item.Recoup)
	require.Equal(t, int64(66274), item.Recoup.AccountID)
	require.Equal(t, "lindaphillipsn905+inv@gmail.com", item.Recoup.AccountName)
	require.InDelta(t, 32.89, item.Recoup.Billed, 0.0001)
	require.Equal(t, int64(3176), item.Recoup.Requests)
	require.InDelta(t, 45.0, item.Recoup.Cost, 0.0001)
	require.InDelta(t, -12.11, item.Recoup.Profit, 0.0001)
}

func TestFinanceScopeAccountIDParsesJSONNumber(t *testing.T) {
	id, ok := FinanceScopeAccountID(map[string]any{"account_id": "66274"})
	require.True(t, ok)
	require.Equal(t, int64(66274), id)
}

func TestNormalizeExpenseListFilterTreatsAllAsUnfiltered(t *testing.T) {
	filter := normalizeExpenseListFilter(ExpenseListFilter{Status: "all", Keyword: "  linda  ", AccountID: -3})
	require.Equal(t, "", filter.Status)
	require.Equal(t, "linda", filter.Keyword)
	require.Equal(t, int64(0), filter.AccountID)
	require.Equal(t, 1, filter.Page)
	require.Equal(t, 20, filter.PageSize)
}
