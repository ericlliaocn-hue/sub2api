//go:build unit

package service

import (
	"testing"
)

// TestBuildUsageBillingCommand_SubscriptionAppliesRateMultiplier locks in the fix
// that subscription-mode billing honours the group (and any user-specific) rate
// multiplier — i.e. cmd.SubscriptionCost tracks ActualCost (= TotalCost *
// RateMultiplier), not raw TotalCost.
func TestBuildUsageBillingCommand_SubscriptionAppliesRateMultiplier(t *testing.T) {
	t.Parallel()

	groupID := int64(7)
	subID := int64(42)

	tests := []struct {
		name           string
		totalCost      float64
		actualCost     float64
		isSubscription bool
		wantSub        float64
		wantBalance    float64
	}{
		{
			name:           "subscription with 2x multiplier consumes 2x quota",
			totalCost:      1.0,
			actualCost:     2.0,
			isSubscription: true,
			wantSub:        2.0,
			wantBalance:    0,
		},
		{
			name:           "subscription with 0.5x multiplier consumes 0.5x quota",
			totalCost:      1.0,
			actualCost:     0.5,
			isSubscription: true,
			wantSub:        0.5,
			wantBalance:    0,
		},
		{
			name:           "free subscription (multiplier 0) consumes no quota",
			totalCost:      1.0,
			actualCost:     0,
			isSubscription: true,
			wantSub:        0,
			wantBalance:    0,
		},
		{
			name:           "balance billing keeps using ActualCost (regression)",
			totalCost:      1.0,
			actualCost:     2.0,
			isSubscription: false,
			wantSub:        0,
			wantBalance:    2.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &postUsageBillingParams{
				Cost:               &CostBreakdown{TotalCost: tt.totalCost, ActualCost: tt.actualCost},
				User:               &User{ID: 1},
				APIKey:             &APIKey{ID: 2, GroupID: &groupID},
				Account:            &Account{ID: 3},
				Subscription:       &UserSubscription{ID: subID},
				IsSubscriptionBill: tt.isSubscription,
			}

			cmd := buildUsageBillingCommand("req-1", nil, p)
			if cmd == nil {
				t.Fatal("buildUsageBillingCommand returned nil")
			}
			if cmd.SubscriptionCost != tt.wantSub {
				t.Errorf("SubscriptionCost = %v, want %v", cmd.SubscriptionCost, tt.wantSub)
			}
			if cmd.BalanceCost != tt.wantBalance {
				t.Errorf("BalanceCost = %v, want %v", cmd.BalanceCost, tt.wantBalance)
			}
		})
	}
}

// Phase 5: 请求开始时固定的账号倍率版本快照必须原样进入扣费命令，
// 扣费不得在请求结束时重读当前倍率。
func TestBuildUsageBillingCommand_CarriesRateVersionSnapshot(t *testing.T) {
	t.Parallel()

	versionID := int64(77)
	source := "manual"
	snapshot := map[string]any{"applied_multiplier": 1.5}
	p := &postUsageBillingParams{
		Cost:                  &CostBreakdown{TotalCost: 1.0, ActualCost: 1.0},
		User:                  &User{ID: 1},
		APIKey:                &APIKey{ID: 2},
		Account:               &Account{ID: 3},
		AccountRateMultiplier: 1.5,
		AccountRateVersionID:  &versionID,
		AccountRateSource:     source,
		AccountRateSnapshot:   snapshot,
	}

	cmd := buildUsageBillingCommand("req-rate-version", nil, p)
	if cmd == nil {
		t.Fatal("buildUsageBillingCommand returned nil")
	}
	if cmd.AccountRateVersionID == nil || *cmd.AccountRateVersionID != versionID {
		t.Errorf("AccountRateVersionID = %v, want %d", cmd.AccountRateVersionID, versionID)
	}
	if cmd.AccountRateSource != source {
		t.Errorf("AccountRateSource = %q, want %q", cmd.AccountRateSource, source)
	}
	if cmd.AccountRateMultiplier != 1.5 {
		t.Errorf("AccountRateMultiplier = %v, want 1.5", cmd.AccountRateMultiplier)
	}
	if cmd.AccountRateSnapshot["applied_multiplier"] != 1.5 {
		t.Errorf("AccountRateSnapshot lost, got %v", cmd.AccountRateSnapshot)
	}
}
