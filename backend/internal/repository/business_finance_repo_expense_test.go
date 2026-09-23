package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAttachExpenseRecoupsFillsAccountBilled(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &businessFinanceRepository{db: db}

	start := time.Date(2026, 9, 20, 11, 26, 0, 0, time.UTC)
	end := time.Date(2026, 9, 20, 15, 0, 0, 0, time.UTC)
	items := []service.ExpenseEntry{
		{
			ID:           1,
			Name:         "linda",
			Amount:       45,
			ExchangeRate: 1,
			OccurredAt:   start,
			PeriodStart:  &start,
			PeriodEnd:    &end,
			Scope:        map[string]any{"account_id": float64(66274)},
			Status:       "active",
		},
		{
			ID:         2,
			Name:       "server",
			Amount:     100,
			OccurredAt: start,
			Scope:      map[string]any{},
			Status:     "active",
		},
	}

	mock.ExpectQuery(`UNNEST`).
		WillReturnRows(sqlmock.NewRows([]string{"slot", "account_id", "name", "billed", "requests"}).
			AddRow(0, int64(66274), "lindaphillipsn905+inv@gmail.com", 32.89, int64(3176)))

	require.NoError(t, repo.attachExpenseRecoups(context.Background(), items))
	require.NoError(t, mock.ExpectationsWereMet())
	require.NotNil(t, items[0].Recoup)
	require.Equal(t, int64(66274), items[0].Recoup.AccountID)
	require.Equal(t, "lindaphillipsn905+inv@gmail.com", items[0].Recoup.AccountName)
	require.InDelta(t, 32.89, items[0].Recoup.Billed, 0.0001)
	require.InDelta(t, -12.11, items[0].Recoup.Profit, 0.0001)
	require.Nil(t, items[1].Recoup)
}

func TestLikePatternEscapesWildcards(t *testing.T) {
	require.Equal(t, "", likePattern("  "))
	require.Equal(t, `%linda%`, likePattern("linda"))
	require.Equal(t, `%100\%%`, likePattern("100%"))
}
