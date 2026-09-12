//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Patterns are kept short and distinctive rather than spelling out whole
// statements: the balance path routes through the recharge-bonus wallet
// helpers, so a test that pins every character rots on unrelated edits.
const (
	lockUserBalanceSQL       = `SELECT balance FROM users WHERE id = \$1 AND deleted_at IS NULL FOR UPDATE`
	lockUserBalanceFrozenSQL = `SELECT balance, COALESCE\(frozen_balance,0\) FROM users WHERE id=\$1`
	expiredBonusLotsSQL      = `(?s)FROM recharge_bonus_grants.+expires_at <= NOW\(\)`
	activeBonusLotsSQL       = `(?s)SELECT id, remaining_amount.+FROM recharge_bonus_grants.+expires_at > NOW\(\)`
	activeBonusTotalSQL      = `SELECT COALESCE\(SUM\(remaining_amount\), 0\)`
	bonusHoldAllocationsSQL  = `FROM recharge_bonus_hold_allocations a`
	deleteBonusHoldSQL       = `DELETE FROM recharge_bonus_hold_allocations`
	insertBalanceLedgerSQL   = `INSERT INTO user_balance_ledgers`
	balanceDeductSQL         = `(?s)UPDATE users SET balance = balance - \$1, updated_at = NOW\(\)\s+WHERE id = \$2 AND deleted_at IS NULL RETURNING balance`
	reserveBatchImageHoldSQL = `(?s)UPDATE users\s+SET balance = balance - \$1,\s+frozen_balance = COALESCE\(frozen_balance, 0\) \+ \$1,\s+updated_at = NOW\(\)\s+WHERE id = \$2 AND deleted_at IS NULL\s+RETURNING balance, frozen_balance`
	captureBatchImageHoldSQL = `(?s)UPDATE users\s+SET balance = balance \+ \$2,\s+frozen_balance = COALESCE\(frozen_balance, 0\) - \$1,.+WHERE id = \$3 AND deleted_at IS NULL AND COALESCE\(frozen_balance, 0\) >= \$1`
	releaseBatchImageHoldSQL = `(?s)UPDATE users\s+SET balance = balance \+ \$3,\s+frozen_balance = COALESCE\(frozen_balance, 0\) - \$1,.+WHERE id = \$2 AND deleted_at IS NULL AND COALESCE\(frozen_balance, 0\) >= \$1`
	holdClaimDedupSQL        = `SELECT 1\s+FROM usage_billing_dedup\s+WHERE request_id = \$1 AND api_key_id = \$2`
	holdClaimArchiveSQL      = `SELECT 1\s+FROM usage_billing_dedup_archive\s+WHERE request_id = \$1 AND api_key_id = \$2`
)

func newBillingTx(t *testing.T) (context.Context, *sql.Tx, sqlmock.Sqlmock, func()) {
	t.Helper()
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	return ctx, tx, mock, func() { _ = db.Close() }
}

func bonusRows() *sqlmock.Rows { return sqlmock.NewRows([]string{"id", "remaining_amount"}) }

// expectBonusExpirySweep mocks ExpireRechargeBonusLots for a user with nothing
// expired, which short-circuits before it touches users.balance.
func expectBonusExpirySweep(mock sqlmock.Sqlmock, userID int64, balance float64) {
	mock.ExpectQuery(lockUserBalanceSQL).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(balance))
	mock.ExpectQuery(expiredBonusLotsSQL).WithArgs(userID).WillReturnRows(bonusRows())
}

func expectActiveBonusTotal(mock sqlmock.Sqlmock, userID int64, total float64) {
	mock.ExpectQuery(activeBonusTotalSQL).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(total))
}

func expectBonusHoldSettlement(mock sqlmock.Sqlmock, batchID string, userID int64) {
	mock.ExpectQuery(bonusHoldAllocationsSQL).
		WithArgs(batchID, userID).
		WillReturnRows(sqlmock.NewRows([]string{"grant_id", "allocated_amount", "expires_at"}))
	mock.ExpectExec(deleteBonusHoldSQL).
		WithArgs(batchID, userID).
		WillReturnResult(sqlmock.NewResult(0, 0))
}

func expectLedgerInsert(mock sqlmock.Sqlmock) {
	mock.ExpectExec(insertBalanceLedgerSQL).WillReturnResult(sqlmock.NewResult(1, 1))
}

func TestDeductUsageBillingBalance_ReportsSufficientWhenBalanceCovers(t *testing.T) {
	ctx, tx, mock, closeDB := newBillingTx(t)
	defer closeDB()

	expectBonusExpirySweep(mock, 42, 10.0)
	mock.ExpectQuery(lockUserBalanceSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(10.0))
	expectActiveBonusTotal(mock, 42, 0)
	mock.ExpectQuery(activeBonusLotsSQL).WithArgs(int64(42)).WillReturnRows(bonusRows())
	mock.ExpectQuery(balanceDeductSQL).
		WithArgs(2.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(7.5))
	expectActiveBonusTotal(mock, 42, 0)
	expectLedgerInsert(mock)
	mock.ExpectCommit()

	newBalance, sufficient, err := deductUsageBillingBalance(ctx, tx, 42, 2.5, "req-sufficient")
	require.NoError(t, err)
	require.True(t, sufficient)
	require.InDelta(t, 7.5, newBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingBalance_ReportsOverdraftWhenBalanceFallsShort(t *testing.T) {
	ctx, tx, mock, closeDB := newBillingTx(t)
	defer closeDB()

	expectBonusExpirySweep(mock, 42, 5.0)
	mock.ExpectQuery(lockUserBalanceSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(5.0))
	expectActiveBonusTotal(mock, 42, 0)
	mock.ExpectQuery(activeBonusLotsSQL).WithArgs(int64(42)).WillReturnRows(bonusRows())
	mock.ExpectQuery(balanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-5.0))
	expectActiveBonusTotal(mock, 42, 0)
	expectLedgerInsert(mock)
	mock.ExpectCommit()

	newBalance, sufficient, err := deductUsageBillingBalance(ctx, tx, 42, 10, "req-overdraft")
	require.NoError(t, err)
	require.False(t, sufficient)
	require.InDelta(t, -5.0, newBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffects_FlagsBalanceOverdraft(t *testing.T) {
	ctx, tx, mock, closeDB := newBillingTx(t)
	defer closeDB()

	expectBonusExpirySweep(mock, 42, 5.0)
	mock.ExpectQuery(lockUserBalanceSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(5.0))
	expectActiveBonusTotal(mock, 42, 0)
	mock.ExpectQuery(activeBonusLotsSQL).WithArgs(int64(42)).WillReturnRows(bonusRows())
	mock.ExpectQuery(balanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-5.0))
	expectActiveBonusTotal(mock, 42, 0)
	expectLedgerInsert(mock)
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err := (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:      42,
		BalanceCost: 10,
		RequestID:   "req-effects",
	}, result)
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, -5.0, *result.NewBalance, 0.000001)
	require.True(t, result.BalanceOverdrafted)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingBalance_ReturnsUserNotFoundWhenRowMissing(t *testing.T) {
	ctx, tx, mock, closeDB := newBillingTx(t)
	defer closeDB()

	mock.ExpectQuery(lockUserBalanceSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}))
	mock.ExpectRollback()

	_, _, err := deductUsageBillingBalance(ctx, tx, 42, 10, "req-missing")
	require.ErrorIs(t, err, service.ErrUserNotFound)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveUsageBillingBatchImageBalance_MovesAvailableToFrozen(t *testing.T) {
	ctx, tx, mock, closeDB := newBillingTx(t)
	defer closeDB()

	expectBonusExpirySweep(mock, 42, 10.0)
	mock.ExpectQuery(lockUserBalanceFrozenSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(10.0, 0.0))
	expectActiveBonusTotal(mock, 42, 0)
	mock.ExpectQuery(activeBonusLotsSQL).WithArgs(int64(42)).WillReturnRows(bonusRows())
	mock.ExpectQuery(reserveBatchImageHoldSQL).
		WithArgs(2.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(7.5, 2.5))
	expectActiveBonusTotal(mock, 42, 0)
	expectLedgerInsert(mock)
	mock.ExpectCommit()

	result, err := reserveUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, BatchID: "imgbatch_reserve", HoldAmount: 2.5})
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.NotNil(t, result.FrozenBalance)
	require.InDelta(t, 7.5, *result.NewBalance, 0.000001)
	require.InDelta(t, 2.5, *result.FrozenBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveUsageBillingBatchImageBalance_InsufficientBalance(t *testing.T) {
	ctx, tx, mock, closeDB := newBillingTx(t)
	defer closeDB()

	// The shortfall is now caught against the locked row, before any bonus lot
	// is allocated, so nothing after the lock should be queried.
	expectBonusExpirySweep(mock, 42, 5.0)
	mock.ExpectQuery(lockUserBalanceFrozenSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(5.0, 0.0))
	mock.ExpectRollback()

	_, err := reserveUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, BatchID: "imgbatch_short", HoldAmount: 10})
	require.ErrorIs(t, err, service.ErrBatchImageInsufficientBalance)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptureUsageBillingBatchImageBalance_ReleasesRemainder(t *testing.T) {
	ctx, tx, mock, closeDB := newBillingTx(t)
	defer closeDB()

	expectBonusExpirySweep(mock, 42, 9.0)
	mock.ExpectQuery(lockUserBalanceFrozenSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(9.0, 1.0))
	expectActiveBonusTotal(mock, 42, 0)
	expectBonusHoldSettlement(mock, "imgbatch_capture", 42)
	// No bonus was allocated, so the whole unused 0.75 comes back as cash.
	mock.ExpectQuery(captureBatchImageHoldSQL).
		WithArgs(1.0, 0.75, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(9.75, 0.0))
	expectActiveBonusTotal(mock, 42, 0)
	expectLedgerInsert(mock)
	mock.ExpectCommit()

	result, err := captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, BatchID: "imgbatch_capture", HoldAmount: 1, ActualAmount: 0.25})
	require.NoError(t, err)
	require.InDelta(t, 9.75, *result.NewBalance, 0.000001)
	require.InDelta(t, 0.0, *result.FrozenBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptureUsageBillingBatchImageBalance_RejectsActualCostOverHold(t *testing.T) {
	ctx, tx, mock, closeDB := newBillingTx(t)
	defer closeDB()

	mock.ExpectRollback()

	_, err := captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 0.5, ActualAmount: 1})
	require.ErrorIs(t, err, service.ErrBatchImageSettlementCostExceedsHold)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseUsageBillingBatchImageBalance_ReturnsFrozenToAvailable(t *testing.T) {
	ctx, tx, mock, closeDB := newBillingTx(t)
	defer closeDB()

	mock.ExpectQuery(holdClaimDedupSQL).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_release"), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"?column?"}).AddRow(1))
	expectBonusExpirySweep(mock, 42, 9.0)
	mock.ExpectQuery(lockUserBalanceFrozenSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(9.0, 1.0))
	expectActiveBonusTotal(mock, 42, 0)
	expectBonusHoldSettlement(mock, "imgbatch_release", 42)
	mock.ExpectQuery(releaseBatchImageHoldSQL).
		WithArgs(1.0, int64(42), 1.0).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(10.0, 0.0))
	expectActiveBonusTotal(mock, 42, 0)
	expectLedgerInsert(mock)
	mock.ExpectCommit()

	result, err := releaseUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, APIKeyID: 7, BatchID: "imgbatch_release", HoldAmount: 1})
	require.NoError(t, err)
	require.InDelta(t, 10.0, *result.NewBalance, 0.000001)
	require.InDelta(t, 0.0, *result.FrozenBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseUsageBillingBatchImageBalance_SkipsWhenHoldNeverReserved(t *testing.T) {
	ctx, tx, mock, closeDB := newBillingTx(t)
	defer closeDB()

	// Neither the dedup table nor its archive holds a claim, so the job never
	// froze anything. Releasing anyway would mint balance out of other users'
	// frozen funds.
	mock.ExpectQuery(holdClaimDedupSQL).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_phantom"), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(holdClaimArchiveSQL).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_phantom"), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectCommit()

	result, err := releaseUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, APIKeyID: 7, BatchID: "imgbatch_phantom", HoldAmount: 1})
	require.NoError(t, err)
	require.Nil(t, result.NewBalance)
	require.Nil(t, result.FrozenBalance)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}
