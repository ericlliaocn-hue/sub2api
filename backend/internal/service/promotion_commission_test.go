package service

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestReversePromotionCommissionUsesRefundRatio(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("UPDATE promotion_commission_ledger").
		WithArgs(int64(42), 0.25).
		WillReturnResult(sqlmock.NewResult(0, 1))

	plan := &RefundPlan{
		OrderID:      42,
		Order:        &dbent.PaymentOrder{ID: 42, Amount: 100},
		RefundAmount: 25,
	}
	require.NoError(t, reversePromotionCommission(context.Background(), db, plan))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReversePromotionCommissionSkipsInvalidPlan(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, reversePromotionCommission(context.Background(), db, nil))
	require.NoError(t, mock.ExpectationsWereMet())
}
