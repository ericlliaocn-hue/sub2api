package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFinanceScopeAccountIDsMergesSiblings(t *testing.T) {
	ids := FinanceScopeAccountIDs(map[string]any{
		"account_id":  float64(66281),
		"account_ids": []any{float64(66281), float64(66283)},
	})
	require.Equal(t, []int64{66281, 66283}, ids)
}

func TestAccountEmailKeyStripsWorkspaceSuffix(t *testing.T) {
	require.Equal(t, "susannwinans2533652@gmail.com", accountEmailKey("susannwinans2533652@gmail.com-12"))
	require.Equal(t, "beilegedong@gmail.com", accountEmailKey("beilegedong@gmail.com"))
	require.Equal(t, "https://mdkj.lol/", accountEmailKey("https://mdkj.lol/"))
}

func TestBuildProfitCalendarMergesEmailAndExpenseAndTotals(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	start := time.Date(2026, 9, 21, 0, 0, 0, 0, loc)
	end := time.Date(2026, 9, 23, 0, 0, 0, 0, loc)

	calendar := BuildProfitCalendar(
		[]ProfitCalendarAccount{
			{ID: 66276, Name: "susannwinans2533652@gmail.com", Status: "error", CreatedAt: time.Date(2026, 9, 21, 8, 59, 0, 0, loc)},
			{ID: 66277, Name: "susannwinans2533652@gmail.com-12", Status: "error", CreatedAt: time.Date(2026, 9, 21, 11, 26, 0, 0, loc)},
			{ID: 66281, Name: "hrvynro74673@gmail.com", Status: "error", CreatedAt: time.Date(2026, 9, 21, 22, 8, 0, 0, loc)},
			{ID: 66283, Name: "hrvynro74673@gmail.com", Status: "active", CreatedAt: time.Date(2026, 9, 22, 6, 39, 0, 0, loc)},
			{ID: 66288, Name: "barbaragreend523@gmail.com", Status: "active", Schedulable: true, CreatedAt: time.Date(2026, 9, 22, 17, 15, 0, 0, loc)},
			{ID: 66284, Name: "sybilvandenberghe0981395@gmail.com", Status: "active", CreatedAt: time.Date(2026, 9, 22, 10, 0, 0, 0, loc)},
		},
		[]ProfitCalendarExpense{
			{ID: 45, Name: "susannwinans2533652@gmail.com-team5x-12", Amount: 49, ExchangeRate: 1, OccurredAt: time.Date(2026, 9, 21, 8, 59, 0, 0, loc), Scope: map[string]any{"account_id": 66276, "account_ids": []any{66276, 66277}}},
			{ID: 50, Name: "hrvynro74673@gmail.com-team5x", Amount: 60, ExchangeRate: 1, OccurredAt: time.Date(2026, 9, 21, 22, 8, 0, 0, loc), Scope: map[string]any{"account_id": 66281, "account_ids": []any{66281, 66283}}},
			{ID: 56, Name: "barbaragreend523@gmail.com", Amount: 68, ExchangeRate: 1, OccurredAt: time.Date(2026, 9, 22, 17, 15, 0, 0, loc), Scope: map[string]any{"account_id": 66288}},
		},
		[]ProfitCalendarUsage{
			{AccountID: 66276, Requests: 10, OfficialTokens: 1000, OfficialBilling: 20, UserBilling: 69.87},
			{AccountID: 66277, Requests: 2, OfficialTokens: 200, OfficialBilling: 3, UserBilling: 6.39},
			{AccountID: 66281, Requests: 4, OfficialTokens: 400, OfficialBilling: 8, UserBilling: 40.43},
			{AccountID: 66283, Requests: 5, OfficialTokens: 500, OfficialBilling: 9, UserBilling: 40.39},
			{AccountID: 66288, Requests: 8, OfficialTokens: 800, OfficialBilling: 15, UserBilling: 92.21},
			{AccountID: 66284, Requests: 3, OfficialTokens: 300, OfficialBilling: 4, UserBilling: 13.92},
		},
		start.UTC(),
		end.UTC(),
		loc,
	)

	require.Equal(t, "Asia/Shanghai", calendar.Timezone)
	require.Len(t, calendar.Days, 2)
	require.Equal(t, "2026-09-22", calendar.Days[0].Date)
	require.Equal(t, "2026-09-21", calendar.Days[1].Date)

	day21 := calendar.Days[1]
	require.Len(t, day21.Rows, 2)
	require.Equal(t, []int64{66276, 66277}, day21.Rows[0].AccountIDs)
	require.InDelta(t, 49, day21.Rows[0].Cost, 0.001)
	require.InDelta(t, 76.26, day21.Rows[0].UserBilling, 0.001)
	require.InDelta(t, 27.26, day21.Rows[0].Profit, 0.001)
	require.True(t, day21.Rows[0].CostRecorded)
	require.Equal(t, []int64{66281, 66283}, day21.Rows[1].AccountIDs)
	require.InDelta(t, 80.82, day21.Rows[1].UserBilling, 0.001)
	require.InDelta(t, 20.82, day21.Rows[1].Profit, 0.001)

	day22 := calendar.Days[0]
	require.Len(t, day22.Rows, 2)
	require.Equal(t, int64(66284), day22.Rows[0].AccountIDs[0])
	require.False(t, day22.Rows[0].CostRecorded)
	require.InDelta(t, 13.92, day22.Rows[0].Profit, 0.001)
	require.Equal(t, int64(66288), day22.Rows[1].AccountIDs[0])
	require.True(t, day22.Rows[1].Schedulable)
	require.InDelta(t, 24.21, day22.Rows[1].Profit, 0.001)

	require.Equal(t, 4, calendar.Summary.Accounts)
	require.Equal(t, 3, calendar.Summary.Recorded)
	require.Equal(t, 1, calendar.Summary.Unrecorded)
	require.Equal(t, 3, calendar.Summary.Recouped)
	require.Equal(t, 0, calendar.Summary.Short)
	require.InDelta(t, 177, calendar.Summary.Cost, 0.001)
	require.InDelta(t, 263.21, calendar.Summary.UserBilling, 0.001)
}

func TestNormalizeProfitCalendarRangeDefaultsLastSevenDays(t *testing.T) {
	start, end, err := NormalizeProfitCalendarRange(time.Time{}, time.Time{})
	require.NoError(t, err)
	require.True(t, end.After(start))
	require.InDelta(t, float64(7*24*time.Hour), float64(end.Sub(start)), float64(time.Hour))
}
