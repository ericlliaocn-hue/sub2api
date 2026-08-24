package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateAppliedRechargeBonusUsesExactEnabledTier(t *testing.T) {
	t.Parallel()

	tiers := []RechargeBonusTier{
		{PaymentAmount: 10, BonusAmount: 2, Currency: "CNY", Enabled: true},
		{PaymentAmount: 50, BonusAmount: 6, Currency: "CNY", Enabled: true},
		{PaymentAmount: 100, BonusAmount: 12, Currency: "CNY", Enabled: false},
	}

	applied := calculateAppliedRechargeBonus(10, "cny", 1, tiers)
	require.NotNil(t, applied)
	require.Equal(t, float64(10), applied.PaymentAmount)
	require.Equal(t, float64(10), applied.BaseCreditedAmount)
	require.Equal(t, float64(2), applied.BonusAmount)
	require.Equal(t, float64(12), applied.CreditedAmount)

	require.Nil(t, calculateAppliedRechargeBonus(20, "CNY", 1, tiers))
	require.Nil(t, calculateAppliedRechargeBonus(100, "CNY", 1, tiers))
	require.Nil(t, calculateAppliedRechargeBonus(10, "USD", 1, tiers))
}

func TestNormalizeRechargeBonusTiersRejectsDuplicatePaymentAmount(t *testing.T) {
	t.Parallel()

	_, err := normalizeRechargeBonusTiers([]RechargeBonusTier{
		{PaymentAmount: 10, BonusAmount: 2, Currency: "cny", Enabled: true},
		{PaymentAmount: 10, BonusAmount: 3, Currency: "CNY", Enabled: true},
	})
	require.ErrorContains(t, err, "duplicate recharge bonus tier")
}

func TestRechargeBonusSnapshotRoundTrip(t *testing.T) {
	t.Parallel()

	snapshot := map[string]any{
		"recharge_bonus": map[string]any{
			"payment_amount":       50,
			"base_credited_amount": 50,
			"bonus_amount":         6,
			"credited_amount":      56,
			"currency":             "CNY",
			"balance_multiplier":   1,
		},
	}

	applied, ok := rechargeBonusFromSnapshot(snapshot)
	require.True(t, ok)
	require.Equal(t, float64(50), applied.PaymentAmount)
	require.Equal(t, float64(6), applied.BonusAmount)
	require.Equal(t, float64(56), applied.CreditedAmount)
}
