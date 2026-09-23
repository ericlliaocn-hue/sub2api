package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestGetUsagePressureSnapshot(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := newUsageLogRepositoryWithSQL(nil, db)

	now := time.Date(2026, 9, 20, 21, 0, 0, 0, time.UTC)
	todayStart := time.Date(2026, 9, 20, 16, 0, 0, 0, time.UTC)
	t5 := now.Add(-5 * time.Minute)
	t15 := now.Add(-15 * time.Minute)
	t60 := now.Add(-60 * time.Minute)
	peakHour := time.Date(2026, 9, 20, 6, 0, 0, 0, time.UTC)
	lastAt := now.Add(-2 * time.Minute)

	windowCols := []string{
		"r5", "u5", "a5", "b5",
		"r15", "u15", "a15", "b15",
		"r60", "u60", "a60", "b60",
		"r_today", "u_today", "a_today", "b_today",
	}
	mock.ExpectQuery(`usage_pressure_windows`).
		WithArgs(todayStart, t5, t15, t60, now).
		WillReturnRows(sqlmock.NewRows(windowCols).AddRow(
			4, 2, 1, 1.2,
			10, 6, 2, 4.21,
			30, 8, 3, 12.5,
			80, 12, 4, 40.0,
		))

	mock.ExpectQuery(`usage_pressure_peak_hour`).
		WithArgs(todayStart, now, "Asia/Shanghai").
		WillReturnRows(sqlmock.NewRows([]string{"hour_at", "requests", "users", "billed"}).
			AddRow(peakHour, 20, 9, 50.0))

	actorCols := []string{"id", "name", "requests", "billed", "last_at"}
	mock.ExpectQuery(`usage_pressure_users`).
		WithArgs(t15, now).
		WillReturnRows(sqlmock.NewRows(actorCols).AddRow(int64(163), "ericlliao@test.com", int64(5), 2.1, lastAt))

	mock.ExpectQuery(`usage_pressure_accounts`).
		WithArgs(t15, now).
		WillReturnRows(sqlmock.NewRows(actorCols).AddRow(int64(66274), "linda", int64(8), 4.0, lastAt))

	snap, err := repo.GetUsagePressureSnapshot(context.Background(), now, todayStart, "Asia/Shanghai")
	require.NoError(t, err)
	require.Equal(t, int64(6), snap.Windows.M15.Users)
	require.InDelta(t, 4.21, snap.Windows.M15.Billed, 0.0001)
	require.InDelta(t, 40.0, snap.Today.Billed, 0.0001)
	require.NotNil(t, snap.PeakHour)
	require.Equal(t, peakHour, snap.PeakHour.Hour)
	require.InDelta(t, 50.0, snap.PeakHour.Billed, 0.0001)
	require.Len(t, snap.Users, 1)
	require.Equal(t, int64(163), snap.Users[0].ID)
	require.Equal(t, "linda", snap.Accounts[0].Name)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUsagePressureSnapshotEmptyDay(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := newUsageLogRepositoryWithSQL(nil, db)

	now := time.Date(2026, 9, 20, 1, 0, 0, 0, time.UTC)
	todayStart := time.Date(2026, 9, 19, 16, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`usage_pressure_windows`).
		WillReturnRows(sqlmock.NewRows([]string{
			"r5", "u5", "a5", "b5",
			"r15", "u15", "a15", "b15",
			"r60", "u60", "a60", "b60",
			"r_today", "u_today", "a_today", "b_today",
		}))
	mock.ExpectQuery(`usage_pressure_peak_hour`).
		WillReturnRows(sqlmock.NewRows([]string{"hour_at", "requests", "users", "billed"}))
	mock.ExpectQuery(`usage_pressure_users`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "requests", "billed", "last_at"}))
	mock.ExpectQuery(`usage_pressure_accounts`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "requests", "billed", "last_at"}))

	snap, err := repo.GetUsagePressureSnapshot(context.Background(), now, todayStart, "")
	require.NoError(t, err)
	require.Zero(t, snap.Windows.M15.Users)
	require.Nil(t, snap.PeakHour)
	require.Empty(t, snap.Users)
	require.Empty(t, snap.Accounts)
	require.NoError(t, mock.ExpectationsWereMet())
}
