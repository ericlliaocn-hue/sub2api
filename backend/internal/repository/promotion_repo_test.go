package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPromotionReportCalculatesContributionMetrics(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	columns := []string{
		"id", "code", "name", "channel_type", "system", "promoter_name",
		"visits", "new_users", "invited_users", "paying_users", "active_users", "recharge", "revenue",
		"upstream_cost", "bonus_cost", "affiliate_cost", "commission_cost", "payment_fee", "marketing_cost",
	}
	mock.ExpectQuery("WITH attributed AS").
		WithArgs(start, end, service.PromotionReportModeOperation, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows(columns).
			AddRow(1, "OFFICIAL", "官网", "official", true, "", 200, 30, 2, 5, 12, 300, 90, 30, 0, 0, 0, 1, 0).
			AddRow(2, "SEO", "搜索", "seo", true, "", 100, 10, 0, 4, 6, 100, 60, 20, 3, 2, 0, 1, 0).
			AddRow(3, "TG1", "TG 群", "other", false, "Agent", 50, 10, 1, 4, 6, 100, 60, 20, 3, 2, 4, 1, 5))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\)::bigint FROM users").
		WithArgs(start, end).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(51))
	breakdownColumns := []string{"key", "label", "visits", "new_users", "paying_users", "revenue", "extra"}
	mock.ExpectQuery("WITH cohort AS").WillReturnRows(sqlmock.NewRows(breakdownColumns).AddRow("baidu", "baidu", 80, 7, 3, 40, 0))
	mock.ExpectQuery("WITH cohort AS").WillReturnRows(sqlmock.NewRows(breakdownColumns))
	mock.ExpectQuery("WITH cohort AS").WillReturnRows(sqlmock.NewRows(breakdownColumns).AddRow("9", "inviter@example.com", 0, 3, 1, 12, 2.5))
	mock.ExpectQuery("WITH cohort AS").WillReturnRows(sqlmock.NewRows(breakdownColumns))

	report, err := NewPromotionRepository(db).Report(context.Background(), start, end, service.PromotionReportModeOperation)
	require.NoError(t, err)
	require.Len(t, report.Rows, 3)

	tg := report.Rows[2]
	require.Equal(t, "other", tg.AcquisitionClass)
	require.False(t, tg.System)
	require.InDelta(t, 25, tg.Profit, 1e-9)
	require.InDelta(t, 0.5, tg.CAC, 1e-9)
	require.InDelta(t, 15, tg.LTV, 1e-9)
	require.InDelta(t, 5, tg.ROI, 1e-9)
	require.InDelta(t, 0.2, tg.ConversionRate, 1e-9)

	// 四分类固定顺序，0 也要出现
	require.Len(t, report.Classes, 4)
	require.Equal(t, []string{"official", "seo", "invite", "other"}, []string{report.Classes[0].Class, report.Classes[1].Class, report.Classes[2].Class, report.Classes[3].Class})
	require.Equal(t, int64(30), report.Classes[0].NewUsers)
	require.Equal(t, int64(0), report.Classes[2].NewUsers)
	require.Equal(t, int64(10), report.Classes[3].NewUsers)
	require.InDelta(t, 0.6, report.Classes[0].NewUsersShare, 1e-9)

	// 合计与漏人检查
	require.Equal(t, int64(50), report.Totals.NewUsers)
	require.Equal(t, int64(51), report.Totals.RegisteredUsers)
	require.Equal(t, int64(1), report.Totals.UnattributedUsers)
	require.Equal(t, int64(3), report.Totals.InvitedUsers)
	require.Equal(t, int64(350), report.Totals.Visits)

	require.Len(t, report.SEOEngines, 1)
	require.Equal(t, "baidu", report.SEOEngines[0].Key)
	require.Len(t, report.Inviters, 1)
	require.InDelta(t, 2.5, report.Inviters[0].Extra, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAcquisitionDayRange(t *testing.T) {
	// 开区间落在整点 00:00 时不多算一天
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	startDay, endDay := acquisitionDayRange(start, end)
	require.NotEqual(t, startDay, endDay)
	// 不是整点：end 当天要包含
	_, endDay2 := acquisitionDayRange(start, end.Add(time.Hour))
	require.True(t, endDay2 >= endDay)
}
