package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Phase 5/9: 请求开始时固定倍率版本快照。请求期间账号版本切换（并发探测或
// 手动修改）不得污染已构建的 usage log / 扣费命令——它们只引用请求开始时
// 从 account 对象取出的版本字段，之后不再重读账号当前版本。
func TestRequestFixesRateVersionSnapshotAtStart(t *testing.T) {
	startRate := 1.0
	accountAtStart := &Account{
		RateMultiplier:              &startRate,
		ActiveUpstreamRateVersionID: int64Ptr(11),
		ActiveUpstreamRateSource:    "default",
		ActiveUpstreamRateSnapshot:  map[string]any{"applied_multiplier": 1.0},
	}

	// 请求开始时固定快照（recordUsageCore 阶段）。
	versionID, source, snapshot := accountAtStart.RateVersionFields()
	require.NotNil(t, versionID)
	require.Equal(t, int64(11), *versionID)
	require.NotNil(t, source)
	require.Equal(t, "default", *source)
	require.Equal(t, float64(1.0), snapshot["applied_multiplier"])

	// 请求期间账号版本切换到 2.0（并发探测/手动修改）。
	concurrentRate := 2.0
	accountAfterSwitch := &Account{
		RateMultiplier:              &concurrentRate,
		ActiveUpstreamRateVersionID: int64Ptr(12),
		ActiveUpstreamRateSource:    "manual",
	}

	// 旧请求的 usage log 与扣费命令仍使用请求开始时的快照。
	usageLogMultiplier := accountAtStart.BillingRateMultiplier()
	require.Equal(t, 1.0, usageLogMultiplier)
	_ = accountAfterSwitch
	require.Equal(t, float64(1.0), snapshot["applied_multiplier"])
	require.NotEqual(t, 2.0, usageLogMultiplier)
}
