package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Phase 7: 所有统计必须统一使用
//
//	COALESCE(upstream_cost, COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1))
//
// 上游成本优先，未配置时回退账号统计价 × 账号倍率。任何裸的
// "account_stats_cost × account_rate_multiplier"（未包 upstream_cost）都是口径漂移。
func TestReportCostFormulaIsUnifiedWithUpstreamCostPriority(t *testing.T) {
	files := []string{
		"usage_log_repo_stats.go",
		"usage_log_repo_trend.go",
		"usage_log_repo_dashboard.go",
		"dashboard_aggregation_repo.go",
		"business_finance_report_repo.go",
	}
	unprefixed := "COALESCE(upstream_cost, COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1))"
	prefixed := "COALESCE(ul.upstream_cost, COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1))"
	bare := "COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1)"

	for _, name := range files {
		path := filepath.Join(".", name)
		content, err := os.ReadFile(path)
		require.NoError(t, err, "read %s", name)
		sql := string(content)

		// 统一公式（带或不带 ul 前缀）必须至少出现一次。
		require.True(t, strings.Contains(sql, unprefixed) || strings.Contains(sql, prefixed),
			"%s: upstream_cost-first unified formula missing", name)

		// 不允许出现未包 upstream_cost 的裸账号成本公式（去掉统一公式后不得残留裸公式）。
		rest := strings.ReplaceAll(sql, unprefixed, "")
		rest = strings.ReplaceAll(rest, prefixed, "")
		require.NotContains(t, rest, bare, "%s: bare account cost formula without upstream_cost priority", name)
	}
}
