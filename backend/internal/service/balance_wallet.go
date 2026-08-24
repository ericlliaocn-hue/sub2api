package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

const balanceEpsilon = 0.00000001

// BalanceSQLExecutor is implemented by both database/sql transactions and Ent transactional clients.
type BalanceSQLExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func ScanBalanceRow(ctx context.Context, exec BalanceSQLExecutor, query string, args []any, dest ...any) error {
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	return rows.Scan(dest...)
}

type BalanceLedgerEntry struct {
	UserID        int64
	EventType     string
	Amount        float64
	BalanceBefore float64
	BalanceAfter  float64
	BonusBefore   float64
	BonusAfter    float64
	SourceType    string
	SourceID      string
	Description   string
	Metadata      map[string]any
}

func ActiveRechargeBonusTotal(ctx context.Context, exec BalanceSQLExecutor, userID int64) (float64, error) {
	var total float64
	err := ScanBalanceRow(ctx, exec, `
		SELECT COALESCE(SUM(remaining_amount), 0)
		FROM recharge_bonus_grants
		WHERE user_id = $1 AND status = 'active' AND remaining_amount > 0 AND expires_at > NOW()
	`, []any{userID}, &total)
	return total, err
}

// ExpireRechargeBonusLots removes expired, unused bonus from the user's total balance.
// The caller must execute this inside a transaction.
func ExpireRechargeBonusLots(ctx context.Context, exec BalanceSQLExecutor, userID int64) (float64, error) {
	var balanceBefore float64
	if err := ScanBalanceRow(ctx, exec, `SELECT balance FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, []any{userID}, &balanceBefore); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrUserNotFound
		}
		return 0, err
	}

	rows, err := exec.QueryContext(ctx, `
		SELECT id, remaining_amount
		FROM recharge_bonus_grants
		WHERE user_id = $1 AND status = 'active' AND remaining_amount > 0 AND expires_at <= NOW()
		ORDER BY expires_at, id
		FOR UPDATE
	`, userID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	var ids []int64
	var expired float64
	for rows.Next() {
		var id int64
		var remaining float64
		if err := rows.Scan(&id, &remaining); err != nil {
			return 0, err
		}
		ids = append(ids, id)
		expired += remaining
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if expired <= balanceEpsilon {
		return 0, nil
	}

	for _, id := range ids {
		if _, err := exec.ExecContext(ctx, `
			UPDATE recharge_bonus_grants
			SET remaining_amount = 0, status = 'expired', updated_at = NOW()
			WHERE id = $1
		`, id); err != nil {
			return 0, err
		}
	}
	var balanceAfter float64
	if err := ScanBalanceRow(ctx, exec, `
		UPDATE users SET balance = GREATEST(balance - $1, 0), updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL RETURNING balance
	`, []any{expired, userID}, &balanceAfter); err != nil {
		return 0, err
	}
	bonusAfter, err := ActiveRechargeBonusTotal(ctx, exec, userID)
	if err != nil {
		return 0, err
	}
	if err := RecordBalanceLedger(ctx, exec, BalanceLedgerEntry{
		UserID: userID, EventType: "bonus_expired", Amount: balanceAfter - balanceBefore,
		BalanceBefore: balanceBefore, BalanceAfter: balanceAfter,
		BonusBefore: bonusAfter + expired, BonusAfter: bonusAfter,
		SourceType: "bonus_expiry", SourceID: fmt.Sprintf("%d:%d", userID, time.Now().UnixNano()),
		Description: "充值赠送额度到期收回",
	}); err != nil {
		return 0, err
	}
	return expired, nil
}

// ConsumeRechargeBonusLots consumes active lots in earliest-expiry-first order.
// It only mutates lots; callers update users.balance in the same transaction.
func ConsumeRechargeBonusLots(ctx context.Context, exec BalanceSQLExecutor, userID int64, amount float64) (float64, error) {
	if amount <= balanceEpsilon {
		return 0, nil
	}
	rows, err := exec.QueryContext(ctx, `
		SELECT id, remaining_amount
		FROM recharge_bonus_grants
		WHERE user_id = $1 AND status = 'active' AND remaining_amount > 0 AND expires_at > NOW()
		ORDER BY expires_at, id
		FOR UPDATE
	`, userID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	type lot struct {
		id        int64
		remaining float64
	}
	lots := make([]lot, 0)
	for rows.Next() {
		var v lot
		if err := rows.Scan(&v.id, &v.remaining); err != nil {
			return 0, err
		}
		lots = append(lots, v)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	left := amount
	consumed := 0.0
	for _, v := range lots {
		if left <= balanceEpsilon {
			break
		}
		take := math.Min(v.remaining, left)
		newRemaining := v.remaining - take
		status := "active"
		if newRemaining <= balanceEpsilon {
			newRemaining = 0
			status = "consumed"
		}
		if _, err := exec.ExecContext(ctx, `
			UPDATE recharge_bonus_grants
			SET remaining_amount = $1, status = $2, updated_at = NOW()
			WHERE id = $3
		`, newRemaining, status, v.id); err != nil {
			return 0, err
		}
		consumed += take
		left -= take
	}
	return consumed, nil
}

// AllocateRechargeBonusHold reserves FEFO bonus lots for an asynchronous batch hold.
func AllocateRechargeBonusHold(ctx context.Context, exec BalanceSQLExecutor, userID int64, batchID string, amount float64) (float64, error) {
	if amount <= balanceEpsilon {
		return 0, nil
	}
	rows, err := exec.QueryContext(ctx, `
		SELECT id, remaining_amount
		FROM recharge_bonus_grants
		WHERE user_id = $1 AND status = 'active' AND remaining_amount > 0 AND expires_at > NOW()
		ORDER BY expires_at, id FOR UPDATE
	`, userID)
	if err != nil {
		return 0, err
	}
	type lot struct {
		id        int64
		remaining float64
	}
	var lots []lot
	for rows.Next() {
		var v lot
		if err := rows.Scan(&v.id, &v.remaining); err != nil {
			_ = rows.Close()
			return 0, err
		}
		lots = append(lots, v)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	left := amount
	allocated := 0.0
	for _, v := range lots {
		if left <= balanceEpsilon {
			break
		}
		take := math.Min(v.remaining, left)
		newRemaining := v.remaining - take
		status := "active"
		if newRemaining <= balanceEpsilon {
			newRemaining, status = 0, "consumed"
		}
		if _, err := exec.ExecContext(ctx, `UPDATE recharge_bonus_grants SET remaining_amount=$1,status=$2,updated_at=NOW() WHERE id=$3`, newRemaining, status, v.id); err != nil {
			return 0, err
		}
		if _, err := exec.ExecContext(ctx, `
			INSERT INTO recharge_bonus_hold_allocations (batch_id,user_id,grant_id,allocated_amount)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (batch_id,grant_id) DO UPDATE SET allocated_amount = recharge_bonus_hold_allocations.allocated_amount + EXCLUDED.allocated_amount
		`, batchID, userID, v.id, take); err != nil {
			return 0, err
		}
		allocated += take
		left -= take
	}
	return allocated, nil
}

// SettleRechargeBonusHold consumes the first actualAmount from the reserved bonus lots and
// restores only unexpired unused bonus. Allocations are deleted after settlement.
func SettleRechargeBonusHold(ctx context.Context, exec BalanceSQLExecutor, userID int64, batchID string, actualAmount float64) (promoAllocated, promoRestored float64, err error) {
	rows, err := exec.QueryContext(ctx, `
		SELECT a.grant_id, a.allocated_amount, g.expires_at
		FROM recharge_bonus_hold_allocations a
		JOIN recharge_bonus_grants g ON g.id = a.grant_id
		WHERE a.batch_id = $1 AND a.user_id = $2
		ORDER BY g.expires_at, g.id FOR UPDATE OF g
	`, batchID, userID)
	if err != nil {
		return 0, 0, err
	}
	type allocation struct {
		grantID   int64
		amount    float64
		expiresAt time.Time
	}
	var allocations []allocation
	for rows.Next() {
		var a allocation
		if err := rows.Scan(&a.grantID, &a.amount, &a.expiresAt); err != nil {
			_ = rows.Close()
			return 0, 0, err
		}
		allocations = append(allocations, a)
		promoAllocated += a.amount
	}
	if err := rows.Close(); err != nil {
		return 0, 0, err
	}
	actualLeft := actualAmount
	now := time.Now()
	for _, a := range allocations {
		consumed := math.Min(a.amount, math.Max(0, actualLeft))
		unused := a.amount - consumed
		actualLeft -= consumed
		if unused > balanceEpsilon && a.expiresAt.After(now) {
			if _, err := exec.ExecContext(ctx, `
				UPDATE recharge_bonus_grants
				SET remaining_amount = remaining_amount + $1, status = 'active', updated_at = NOW()
				WHERE id = $2
			`, unused, a.grantID); err != nil {
				return 0, 0, err
			}
			promoRestored += unused
		}
	}
	if _, err := exec.ExecContext(ctx, `DELETE FROM recharge_bonus_hold_allocations WHERE batch_id=$1 AND user_id=$2`, batchID, userID); err != nil {
		return 0, 0, err
	}
	return promoAllocated, promoRestored, nil
}

func RecordBalanceLedger(ctx context.Context, exec BalanceSQLExecutor, entry BalanceLedgerEntry) error {
	metadata, err := json.Marshal(entry.Metadata)
	if err != nil {
		return err
	}
	_, err = exec.ExecContext(ctx, `
		INSERT INTO user_balance_ledgers (
			user_id, event_type, amount, balance_before, balance_after,
			bonus_before, bonus_after, source_type, source_id, description, metadata
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb)
		ON CONFLICT (event_type, source_type, source_id)
		WHERE source_type <> '' AND source_id <> '' DO NOTHING
	`, entry.UserID, entry.EventType, entry.Amount, entry.BalanceBefore, entry.BalanceAfter,
		entry.BonusBefore, entry.BonusAfter, entry.SourceType, entry.SourceID, entry.Description, string(metadata))
	return err
}
