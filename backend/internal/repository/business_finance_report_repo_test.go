package repository

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestRecurringAmountUsesCalendarPeriods(t *testing.T) {
	location := time.UTC
	start := time.Date(2026, time.August, 15, 0, 0, 0, 0, location)
	end := time.Date(2026, time.September, 1, 0, 0, 0, 0, location)

	monthly := recurringAmount("monthly", time.Date(2026, time.August, 1, 0, 0, 0, 0, location), nil, start, end)
	if want := 17.0 / 31.0; monthly != want {
		t.Fatalf("monthly prorate = %v, want %v", monthly, want)
	}

	yearly := recurringAmount("yearly", time.Date(2026, time.January, 1, 0, 0, 0, 0, location), nil, start, end)
	if want := 17.0 / 365.0; yearly != want {
		t.Fatalf("yearly prorate = %v, want %v", yearly, want)
	}
}

func TestFinanceScopeMatchesAcrossDimensions(t *testing.T) {
	row := financeUsageRow{Key: "channel-1", GroupID: 7, ChannelID: 11, AccountID: 13, Model: "gpt-4o"}
	cases := []struct {
		name  string
		scope map[string]any
		match bool
	}{
		{name: "global", scope: map[string]any{}, match: true},
		{name: "group", scope: map[string]any{"group_id": float64(7)}, match: true},
		{name: "channel", scope: map[string]any{"channel_id": "11"}, match: true},
		{name: "account and model", scope: map[string]any{"account_id": float64(13), "model": "gpt-4o"}, match: true},
		{name: "different group", scope: map[string]any{"group_id": float64(8)}, match: false},
		{name: "unknown key", scope: map[string]any{"tenant_id": float64(1)}, match: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := financeScopeMatches(tc.scope, row, "channel"); got != tc.match {
				t.Fatalf("scope match = %v, want %v", got, tc.match)
			}
		})
	}
}

func TestAggregateFinanceRowsAppliesScopedCostBeforeGrouping(t *testing.T) {
	rows := []financeUsageRow{
		{Key: "channel-1", Name: "渠道一", GroupID: 7, ChannelID: 11, Revenue: 10, Direct: 5, Requests: 1},
		{Key: "channel-1", Name: "渠道一", GroupID: 8, ChannelID: 11, Revenue: 20, Direct: 10, Requests: 2},
	}
	components := []financeCostComponent{{
		FinanceCostComponent: service.FinanceCostComponent{
			Name: "分组七服务器", Amount: 30, AllocationMethod: "revenue_share",
		},
		Scope: map[string]any{"group_id": float64(7)},
	}}
	result := aggregateFinanceRows(rows, components, 3, 0, 30, 15, "channel")
	if len(result) != 1 {
		t.Fatalf("aggregated rows = %d, want 1", len(result))
	}
	if result[0].OperatingCost != 10 {
		t.Fatalf("scoped operating cost = %v, want 10", result[0].OperatingCost)
	}
	if result[0].TotalCost != 25 {
		t.Fatalf("total cost = %v, want 25", result[0].TotalCost)
	}
}
