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
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	columns := []string{
		"id", "code", "name", "channel_type", "promoter_name",
		"new_users", "paying_users", "active_users", "recharge", "revenue",
		"upstream_cost", "bonus_cost", "affiliate_cost", "commission_cost", "payment_fee", "marketing_cost",
	}
	mock.ExpectQuery("WITH attributed AS").
		WithArgs(start, end, service.PromotionReportModeOperation).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(
			1, "SEO_CN", "SEO", "SEO", "Agent",
			10, 4, 6, 100, 60,
			20, 3, 2, 4, 1, 5,
		))

	report, err := NewPromotionRepository(db).Report(context.Background(), start, end, service.PromotionReportModeOperation)
	require.NoError(t, err)
	require.Len(t, report.Rows, 1)
	row := report.Rows[0]
	require.InDelta(t, 25, row.Profit, 1e-9)
	require.InDelta(t, 0.5, row.CAC, 1e-9)
	require.InDelta(t, 15, row.LTV, 1e-9)
	require.InDelta(t, 5, row.ROI, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}
