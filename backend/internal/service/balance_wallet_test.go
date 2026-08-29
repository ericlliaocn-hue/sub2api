package service

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestConsumeRechargeBonusLotsUsesEarliestExpiryFirst(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, remaining_amount
		FROM recharge_bonus_grants
		WHERE user_id = $1 AND status = 'active' AND remaining_amount > 0 AND expires_at > NOW()
		ORDER BY expires_at, id
		FOR UPDATE
	`)).WithArgs(int64(7)).WillReturnRows(
		sqlmock.NewRows([]string{"id", "remaining_amount"}).
			AddRow(int64(11), 2.0).
			AddRow(int64(12), 3.0),
	)
	mock.ExpectExec(regexp.QuoteMeta(`
			UPDATE recharge_bonus_grants
			SET remaining_amount = $1, status = $2, updated_at = NOW()
			WHERE id = $3
		`)).WithArgs(float64(0), "consumed", int64(11)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`
			UPDATE recharge_bonus_grants
			SET remaining_amount = $1, status = $2, updated_at = NOW()
			WHERE id = $3
		`)).WithArgs(2.5, "active", int64(12)).WillReturnResult(sqlmock.NewResult(0, 1))

	consumed, err := ConsumeRechargeBonusLots(context.Background(), db, 7, 2.5)
	require.NoError(t, err)
	require.InDelta(t, 2.5, consumed, balanceEpsilon)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExpireRechargeBonusLotsReclaimsBalanceAndRecordsLedger(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT balance FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`)).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(10.0))
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, remaining_amount
		FROM recharge_bonus_grants
		WHERE user_id = $1 AND status = 'active' AND remaining_amount > 0 AND expires_at <= NOW()
		ORDER BY expires_at, id
		FOR UPDATE
	`)).WithArgs(int64(9)).WillReturnRows(
		sqlmock.NewRows([]string{"id", "remaining_amount"}).AddRow(int64(21), 1.5).AddRow(int64(22), 0.5),
	)
	for _, grantID := range []int64{21, 22} {
		mock.ExpectExec(regexp.QuoteMeta(`
			UPDATE recharge_bonus_grants
			SET remaining_amount = 0, status = 'expired', updated_at = NOW()
			WHERE id = $1
		`)).WithArgs(grantID).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectQuery(regexp.QuoteMeta(`
		UPDATE users SET balance = GREATEST(balance - $1, 0), updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL RETURNING balance
	`)).WithArgs(2.0, int64(9)).WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(8.0))
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT COALESCE(SUM(remaining_amount), 0)
		FROM recharge_bonus_grants
		WHERE user_id = $1 AND status = 'active' AND remaining_amount > 0 AND expires_at > NOW()
	`)).WithArgs(int64(9)).WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0.75))
	mock.ExpectExec("INSERT INTO user_balance_ledgers").
		WithArgs(int64(9), "bonus_expired", -2.0, 10.0, 8.0, 2.75, 0.75, "bonus_expiry", sqlmock.AnyArg(), "赠送余额到期收回", "null").
		WillReturnResult(sqlmock.NewResult(1, 1))

	expired, err := ExpireRechargeBonusLots(context.Background(), db, 9)
	require.NoError(t, err)
	require.InDelta(t, 2.0, expired, balanceEpsilon)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettleRechargeBonusHoldRestoresOnlyUnexpiredUnusedAmount(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT a.grant_id, a.allocated_amount, g.expires_at
		FROM recharge_bonus_hold_allocations a
		JOIN recharge_bonus_grants g ON g.id = a.grant_id
		WHERE a.batch_id = $1 AND a.user_id = $2
		ORDER BY g.expires_at, g.id FOR UPDATE OF g
	`)).WithArgs("batch-1", int64(5)).WillReturnRows(
		sqlmock.NewRows([]string{"grant_id", "allocated_amount", "expires_at"}).
			AddRow(int64(31), 2.0, now.Add(time.Hour)).
			AddRow(int64(32), 3.0, now.Add(-time.Hour)),
	)
	mock.ExpectExec(regexp.QuoteMeta(`
				UPDATE recharge_bonus_grants
				SET remaining_amount = remaining_amount + $1, status = 'active', updated_at = NOW()
				WHERE id = $2
			`)).WithArgs(1.0, int64(31)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM recharge_bonus_hold_allocations WHERE batch_id=$1 AND user_id=$2`)).
		WithArgs("batch-1", int64(5)).WillReturnResult(sqlmock.NewResult(0, 2))

	allocated, restored, err := SettleRechargeBonusHold(context.Background(), db, 5, "batch-1", 1.0)
	require.NoError(t, err)
	require.InDelta(t, 5.0, allocated, balanceEpsilon)
	require.InDelta(t, 1.0, restored, balanceEpsilon)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRecordBalanceLedgerTreatsDuplicateSourceAsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectExec("INSERT INTO user_balance_ledgers").
		WithArgs(int64(3), "usage_charge", -0.25, 2.0, 1.75, 1.0, 0.75, "usage_request", "req-1", "模型调用扣费", `{"model":"gpt-5.6-luna"}`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = RecordBalanceLedger(context.Background(), db, BalanceLedgerEntry{
		UserID: 3, EventType: "usage_charge", Amount: -0.25,
		BalanceBefore: 2, BalanceAfter: 1.75, BonusBefore: 1, BonusAfter: 0.75,
		SourceType: "usage_request", SourceID: "req-1", Description: "模型调用扣费",
		Metadata: map[string]any{"model": "gpt-5.6-luna"},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
