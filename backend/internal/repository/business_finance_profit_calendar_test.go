package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestGetProfitCalendarLoadsSiblingsAndBuildsDays(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &businessFinanceRepository{db: db}
	loc := time.FixedZone("CST", 8*3600)
	start := time.Date(2026, 9, 21, 0, 0, 0, 0, loc).UTC()
	end := time.Date(2026, 9, 22, 0, 0, 0, 0, loc).UTC()

	mock.ExpectQuery(`created_at >= \$1 AND created_at < \$2`).
		WithArgs(start, end).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "status", "schedulable", "type", "created_at"}).
			AddRow(int64(66281), "hrvynro74673@gmail.com", "error", false, "oauth", time.Date(2026, 9, 21, 22, 8, 0, 0, loc)))

	mock.ExpectQuery(`SELECT id, name, amount, exchange_rate_to_billing_unit, occurred_at, scope\s+FROM business_expenses`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "amount", "exchange_rate_to_billing_unit", "occurred_at", "scope"}).
			AddRow(int64(50), "hrvynro74673@gmail.com-team5x", 60.0, 1.0, time.Date(2026, 9, 21, 22, 8, 0, 0, loc), []byte(`{"account_id":66281,"account_ids":[66281,66283]}`)))

	mock.ExpectQuery(`SELECT id, name, status, schedulable, type, created_at\s+FROM accounts\s+WHERE deleted_at IS NULL AND id = ANY`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "status", "schedulable", "type", "created_at"}).
			AddRow(int64(66283), "hrvynro74673@gmail.com", "active", false, "oauth", time.Date(2026, 9, 22, 6, 39, 0, 0, loc)))

	mock.ExpectQuery(`SELECT account_id`).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "requests", "official_tokens", "official_billing", "user_billing"}).
			AddRow(int64(66281), int64(4), int64(400), 8.0, 40.43).
			AddRow(int64(66283), int64(5), int64(500), 9.0, 40.39))

	calendar, err := repo.GetProfitCalendar(context.Background(), start, end)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.NotNil(t, calendar)
	require.Equal(t, 1, calendar.Summary.Accounts)
	require.InDelta(t, 80.82, calendar.Summary.UserBilling, 0.001)
	require.InDelta(t, 20.82, calendar.Summary.Profit, 0.001)
	require.Equal(t, []int64{66281, 66283}, calendar.Days[0].Rows[0].AccountIDs)
}
