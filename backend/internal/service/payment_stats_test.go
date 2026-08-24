//go:build unit

package service

import (
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestComputeBasicStatsGroupsAmountsByCurrency(t *testing.T) {
	t.Parallel()

	todayStart := time.Date(2026, time.July, 25, 0, 0, 0, 0, time.UTC)
	yesterday := todayStart.Add(-time.Hour)
	today := todayStart.Add(time.Hour)
	orders := []*dbent.PaymentOrder{
		paymentStatsTestOrder(1, "alice@example.com", "CNY", 10, &today),
		paymentStatsTestOrder(2, "bob@example.com", "USD", 10, &today),
		paymentStatsTestOrder(1, "alice@example.com", "CNY", 5, &yesterday),
	}

	stats := &DashboardStats{}
	computeBasicStats(stats, orders, todayStart)

	require.Equal(t, CurrencyAmounts{"CNY": 15, "USD": 10}, stats.TotalAmount)
	require.Equal(t, CurrencyAmounts{"CNY": 10, "USD": 10}, stats.TodayAmount)
	require.Equal(t, CurrencyAmounts{"CNY": 7.5, "USD": 10}, stats.AvgAmount)
	require.Equal(t, 3, stats.TotalCount)
	require.Equal(t, 2, stats.TodayCount)
}

func TestPaymentDashboardBreakdownsGroupAmountsAndRankingsByCurrency(t *testing.T) {
	t.Parallel()

	firstDay := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	secondDay := firstDay.AddDate(0, 0, 1)
	orders := []*dbent.PaymentOrder{
		paymentStatsTestOrder(1, "alice@example.com", "CNY", 5.555, &firstDay),
		paymentStatsTestOrder(2, "bob@example.com", "CNY", 10, &firstDay),
		paymentStatsTestOrder(1, "alice@example.com", "USD", 20, &secondDay),
		paymentStatsTestOrder(2, "bob@example.com", "USD", 10, &secondDay),
	}
	orders[0].PaymentType = "stripe"
	orders[1].PaymentType = "stripe"
	orders[2].PaymentType = "stripe"
	orders[3].PaymentType = "alipay"

	daily := buildDailySeries(orders, firstDay.AddDate(0, 0, -1), 2)
	require.Equal(t, []DailyStats{
		{Date: "2026-07-24", Amount: CurrencyAmounts{"CNY": 15.56}, Count: 2},
		{Date: "2026-07-25", Amount: CurrencyAmounts{"USD": 30}, Count: 2},
	}, daily)

	methods := buildMethodDistribution(orders)
	require.Equal(t, []PaymentMethodStat{
		{Type: "alipay", Amount: CurrencyAmounts{"USD": 10}, Count: 1},
		{Type: "stripe", Amount: CurrencyAmounts{"CNY": 15.56, "USD": 20}, Count: 3},
	}, methods)

	users := buildTopUsers(orders)
	require.Equal(t, TopUsersByCurrency{
		"CNY": {
			{UserID: 2, Email: "bob@example.com", Amount: 10},
			{UserID: 1, Email: "alice@example.com", Amount: 5.56},
		},
		"USD": {
			{UserID: 1, Email: "alice@example.com", Amount: 20},
			{UserID: 2, Email: "bob@example.com", Amount: 10},
		},
	}, users)
}

func TestBuildRechargeBonusStatsUsesCompletedBalanceOrdersOnly(t *testing.T) {
	t.Parallel()

	completed := &dbent.PaymentOrder{
		Status:    OrderStatusCompleted,
		OrderType: payment.OrderTypeBalance,
		ProviderSnapshot: map[string]any{
			"recharge_bonus": map[string]any{
				"payment_amount":  10,
				"bonus_amount":    2,
				"credited_amount": 12,
				"currency":        "CNY",
			},
		},
	}
	secondCompleted := &dbent.PaymentOrder{
		Status:    OrderStatusCompleted,
		OrderType: payment.OrderTypeBalance,
		ProviderSnapshot: map[string]any{
			"recharge_bonus": map[string]any{
				"payment_amount":  10,
				"bonus_amount":    2,
				"credited_amount": 12,
				"currency":        "CNY",
			},
		},
	}
	paidOnly := &dbent.PaymentOrder{Status: OrderStatusPaid, OrderType: payment.OrderTypeBalance, ProviderSnapshot: completed.ProviderSnapshot}
	subscription := &dbent.PaymentOrder{Status: OrderStatusCompleted, OrderType: payment.OrderTypeSubscription, ProviderSnapshot: completed.ProviderSnapshot}
	legacy := &dbent.PaymentOrder{Status: OrderStatusCompleted, OrderType: payment.OrderTypeBalance}

	stats := buildRechargeBonusStats([]*dbent.PaymentOrder{completed, secondCompleted, paidOnly, subscription, legacy})

	require.Equal(t, 2, stats.OrderCount)
	require.Equal(t, CurrencyAmounts{"CNY": 20}, stats.PaymentAmounts)
	require.Equal(t, float64(4), stats.BonusAmount)
	require.Equal(t, float64(24), stats.CreditedAmount)
	require.Equal(t, []RechargeBonusTierStats{{
		Currency:            "CNY",
		PaymentAmount:       10,
		BonusAmount:         2,
		OrderCount:          2,
		TotalPaymentAmount:  20,
		TotalBonusAmount:    4,
		TotalCreditedAmount: 24,
	}}, stats.Tiers)
}

func paymentStatsTestOrder(userID int64, email, currency string, amount float64, paidAt *time.Time) *dbent.PaymentOrder {
	return &dbent.PaymentOrder{
		UserID:           userID,
		UserEmail:        email,
		PayAmount:        amount,
		PaidAt:           paidAt,
		ProviderSnapshot: map[string]any{"currency": currency},
	}
}
