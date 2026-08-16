package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestApplyManualUpstreamCost_LunaShortContext(t *testing.T) {
	profile := UpstreamCostVersion{
		ID: 17, Model: "gpt-5.6-luna",
		ShortPrices:          UpstreamCostPrices{Input: 1, CacheRead: 0.1, CacheWrite: 1, Output: 6},
		LongContextThreshold: 272000,
		LongPrices:           UpstreamCostPrices{Input: 2, CacheRead: 0.2, CacheWrite: 2, Output: 9},
		DeclaredMultiplier:   0.035,
		BalanceUnitCost:      1.018,
		EffectiveFrom:        time.Date(2026, 8, 15, 4, 33, 0, 0, time.FixedZone("CST", 8*60*60)),
	}
	account := &Account{Extra: map[string]any{
		UpstreamCostProfilesExtraKey: map[string]any{"gpt-5.6-luna": profile.ExtraSnapshot()},
	}}
	usageLog := &UsageLog{TotalCost: 0.14}

	applyManualUpstreamCost(usageLog, account, "gpt-5.6-luna", "gpt-5.6-luna", UsageTokens{
		InputTokens: 100_000, OutputTokens: 100_000,
	})

	require.NotNil(t, usageLog.UpstreamCost)
	require.InDelta(t, 0.024941, *usageLog.UpstreamCost, 1e-12)
	require.InDelta(t, 0.17815, usageLog.UpstreamCostSnapshot["normalized_multiplier"], 1e-12)
	require.Equal(t, int64(17), usageLog.UpstreamCostSnapshot["version_id"])
	require.Equal(t, false, usageLog.UpstreamCostSnapshot["long_context_applied"])
}

func TestApplyManualUpstreamCost_LunaLongContext(t *testing.T) {
	profile := UpstreamCostVersion{
		ID: 18, Model: "gpt-5.6-luna",
		ShortPrices:          UpstreamCostPrices{Input: 1, CacheRead: 0.1, CacheWrite: 1, Output: 6},
		LongContextThreshold: 272000,
		LongPrices:           UpstreamCostPrices{Input: 2, CacheRead: 0.2, CacheWrite: 2, Output: 9},
		DeclaredMultiplier:   0.035, BalanceUnitCost: 1,
		EffectiveFrom: time.Now(),
	}
	account := &Account{Extra: map[string]any{
		UpstreamCostProfilesExtraKey: map[string]any{"gpt-5.6-luna": profile.ExtraSnapshot()},
	}}
	usageLog := &UsageLog{TotalCost: 0.3}

	applyManualUpstreamCost(usageLog, account, "gpt-5.6-luna", "", UsageTokens{
		InputTokens: 300_000, OutputTokens: 100_000,
	})

	require.NotNil(t, usageLog.UpstreamCost)
	require.InDelta(t, 0.0525, *usageLog.UpstreamCost, 1e-12)
	require.InDelta(t, 0.175, usageLog.UpstreamCostSnapshot["normalized_multiplier"], 1e-12)
	require.Equal(t, true, usageLog.UpstreamCostSnapshot["long_context_applied"])
}

func TestApplyManualUpstreamCost_UnconfiguredModelKeepsLegacyFallback(t *testing.T) {
	usageLog := &UsageLog{TotalCost: 10}
	applyManualUpstreamCost(usageLog, &Account{Extra: map[string]any{}}, "gpt-5.6-sol", "", UsageTokens{InputTokens: 1_000_000})
	require.Nil(t, usageLog.UpstreamCost)
	require.Nil(t, usageLog.UpstreamCostSnapshot)
}

func TestValidateUpstreamCostVersionInputRejectsUnsupportedModel(t *testing.T) {
	input := UpstreamCostVersionInput{
		AccountID: 1, Model: "gpt-5.5",
		ShortPrices:     UpstreamCostPrices{Input: 1, Output: 1},
		LongPrices:      UpstreamCostPrices{Input: 1, Output: 1},
		BalanceUnitCost: 1,
	}
	// 计划 §1.1：预置 Luna/Terra 之外允许新增任意模型，非空即合法。
	require.NoError(t, validateUpstreamCostVersionInput(&input))
	require.Equal(t, "gpt-5.5", input.Model)

	// 空模型 / 超长模型名仍拒绝。
	empty := UpstreamCostVersionInput{AccountID: 1, Model: "  ", ShortPrices: UpstreamCostPrices{Input: 1, Output: 1}, LongPrices: UpstreamCostPrices{Input: 1, Output: 1}, BalanceUnitCost: 1}
	require.Error(t, validateUpstreamCostVersionInput(&empty))
}

// Phase 6: 有效倍率版本优先于成本配置 declared_multiplier，且快照记录
// rate_version_id / rate_source / applied_multiplier / official_cost。
func TestApplyManualUpstreamCost_PrefersActiveRateVersion(t *testing.T) {
	profile := UpstreamCostVersion{
		ID: 27, Model: "gpt-5.6-luna",
		ShortPrices:          UpstreamCostPrices{Input: 1, CacheRead: 0.1, CacheWrite: 1, Output: 6},
		LongContextThreshold: 0,
		LongPrices:           UpstreamCostPrices{Input: 1, CacheRead: 0.1, CacheWrite: 1, Output: 6},
		DeclaredMultiplier:   0.035, // fallback，不应被使用
		BalanceUnitCost:      1,
		EffectiveFrom:        time.Now(),
	}
	versionID := int64(31)
	versionRate := 1.5
	source := "upstream_probe"
	usageLog := &UsageLog{
		TotalCost:             1.0,
		AccountRateMultiplier: &versionRate,
		AccountRateVersionID:  &versionID,
		AccountRateSource:     &source,
	}

	applyManualUpstreamCost(usageLog, &Account{Extra: map[string]any{
		UpstreamCostProfilesExtraKey: map[string]any{"gpt-5.6-luna": profile.ExtraSnapshot()},
	}}, "gpt-5.6-luna", "gpt-5.6-luna", UsageTokens{InputTokens: 100_000, OutputTokens: 100_000})

	require.NotNil(t, usageLog.UpstreamCost)
	// 100k in (1) + 100k out (6) = 0.7; × version rate 1.5 = 1.05（未用 declared 0.035）。
	require.InDelta(t, 1.05, *usageLog.UpstreamCost, 1e-12)
	require.Equal(t, float64(1.5), usageLog.UpstreamCostSnapshot["applied_multiplier"])
	require.Equal(t, int64(31), usageLog.UpstreamCostSnapshot["rate_version_id"])
	require.Equal(t, "upstream_probe", usageLog.UpstreamCostSnapshot["rate_source"])
	require.Equal(t, int64(27), usageLog.UpstreamCostSnapshot["price_version_id"])
	require.Equal(t, float64(1.0), usageLog.UpstreamCostSnapshot["official_cost"])
}

// Phase 6: 无倍率版本时回退 declared_multiplier。
func TestApplyManualUpstreamCost_FallsBackToDeclaredMultiplier(t *testing.T) {
	profile := UpstreamCostVersion{
		ID: 28, Model: "gpt-5.6-luna",
		ShortPrices:        UpstreamCostPrices{Input: 1, CacheRead: 0.1, CacheWrite: 1, Output: 6},
		LongPrices:         UpstreamCostPrices{Input: 1, CacheRead: 0.1, CacheWrite: 1, Output: 6},
		DeclaredMultiplier: 2.0,
		BalanceUnitCost:    1,
		EffectiveFrom:      time.Now(),
	}
	usageLog := &UsageLog{TotalCost: 1.0}

	applyManualUpstreamCost(usageLog, &Account{Extra: map[string]any{
		UpstreamCostProfilesExtraKey: map[string]any{"gpt-5.6-luna": profile.ExtraSnapshot()},
	}}, "gpt-5.6-luna", "gpt-5.6-luna", UsageTokens{InputTokens: 100_000, OutputTokens: 100_000})

	require.NotNil(t, usageLog.UpstreamCost)
	require.InDelta(t, 1.4, *usageLog.UpstreamCost, 1e-12)
	require.Equal(t, float64(2.0), usageLog.UpstreamCostSnapshot["applied_multiplier"])
}

// default 版本只表示账号已初始化，尚未取得或手工确认上游倍率；此时必须继续
// 使用成本配置的 declared_multiplier，不能让 default 1.0 放大真实成本。
func TestApplyManualUpstreamCost_DefaultRateVersionFallsBackToDeclaredMultiplier(t *testing.T) {
	profile := UpstreamCostVersion{
		ID: 29, Model: "gpt-5.6-luna",
		ShortPrices:        UpstreamCostPrices{Input: 1, CacheRead: 0.1, CacheWrite: 1, Output: 6},
		LongPrices:         UpstreamCostPrices{Input: 1, CacheRead: 0.1, CacheWrite: 1, Output: 6},
		DeclaredMultiplier: 0.085,
		BalanceUnitCost:    1,
		EffectiveFrom:      time.Now(),
	}
	versionID := int64(32)
	defaultRate := 1.0
	source := "default"
	usageLog := &UsageLog{
		TotalCost:             0.14,
		AccountRateMultiplier: &defaultRate,
		AccountRateVersionID:  &versionID,
		AccountRateSource:     &source,
	}

	applyManualUpstreamCost(usageLog, &Account{Extra: map[string]any{
		UpstreamCostProfilesExtraKey: map[string]any{"gpt-5.6-luna": profile.ExtraSnapshot()},
	}}, "gpt-5.6-luna", "gpt-5.6-luna", UsageTokens{InputTokens: 100_000, OutputTokens: 100_000})

	require.NotNil(t, usageLog.UpstreamCost)
	// 100k in (1) + 100k out (6) = 0.7; × declared 0.085 = 0.0595。
	require.InDelta(t, 0.0595, *usageLog.UpstreamCost, 1e-12)
	require.Equal(t, float64(0.085), usageLog.UpstreamCostSnapshot["applied_multiplier"])
	require.Equal(t, int64(32), usageLog.UpstreamCostSnapshot["rate_version_id"])
	require.Equal(t, "default", usageLog.UpstreamCostSnapshot["rate_source"])
}
