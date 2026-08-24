package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func paymentOrderBaseCreditedAmount(o *dbent.PaymentOrder) float64 {
	if o != nil {
		if bonus, ok := rechargeBonusFromSnapshot(o.ProviderSnapshot); ok && bonus.BaseCreditedAmount > 0 {
			return bonus.BaseCreditedAmount
		}
		return o.Amount
	}
	return 0
}

func (s *PaymentService) reserveRechargeBonusClaimSlot(ctx context.Context, tx *dbent.Tx, userID int64, applied *AppliedRechargeBonus) error {
	if applied == nil || applied.BonusAmount <= 0 {
		return nil
	}
	exec := tx.Client()
	var lockedID int64
	if err := ScanBalanceRow(ctx, exec, `SELECT id FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, []any{userID}, &lockedID); err != nil {
		return err
	}
	var used int
	if err := ScanBalanceRow(ctx, exec, `
		SELECT
			(SELECT COUNT(*) FROM recharge_bonus_grants WHERE user_id = $1 AND campaign_id = $2)
			+
			(SELECT COUNT(*) FROM payment_orders
			 WHERE user_id = $1
			   AND status IN ('PENDING','PAID','RECHARGING')
			   AND COALESCE(provider_snapshot->'recharge_bonus'->>'campaign_id', '') = $2)
	`, []any{userID, applied.CampaignID}, &used); err != nil {
		return err
	}
	if used >= applied.MaxClaimsPerUser {
		return infraerrors.Forbidden("RECHARGE_BONUS_CLAIM_LIMIT", "该充值活动档位已达到每位用户的参与次数上限")
	}
	return nil
}

func (s *PaymentService) fulfillRechargeBonus(ctx context.Context, o *dbent.PaymentOrder, alreadyCredited bool) error {
	applied, ok := rechargeBonusFromSnapshot(o.ProviderSnapshot)
	if !ok || applied.BonusAmount <= 0 {
		return nil
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	exec := tx.Client()

	var existingID int64
	err = ScanBalanceRow(ctx, exec, `SELECT id FROM recharge_bonus_grants WHERE payment_order_id = $1`, []any{o.ID}, &existingID)
	if err == nil {
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	var balanceBefore float64
	if err := ScanBalanceRow(ctx, exec, `SELECT balance FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, []any{o.UserID}, &balanceBefore); err != nil {
		return err
	}
	var claimCount int
	if err := ScanBalanceRow(ctx, exec, `SELECT COUNT(*) FROM recharge_bonus_grants WHERE user_id = $1 AND campaign_id = $2`, []any{o.UserID, applied.CampaignID}, &claimCount); err != nil {
		return err
	}
	if claimCount >= applied.MaxClaimsPerUser {
		return infraerrors.Conflict("RECHARGE_BONUS_CLAIM_LIMIT", "recharge bonus claim limit reached during fulfillment")
	}

	expiresAt := time.Now().Add(time.Duration(applied.ValidityDays) * 24 * time.Hour)
	var grantID int64
	if err := ScanBalanceRow(ctx, exec, `
		INSERT INTO recharge_bonus_grants (
			user_id, payment_order_id, campaign_id, payment_amount, base_credited_amount,
			granted_amount, remaining_amount, currency, expires_at, status
		) VALUES ($1,$2,$3,$4,$5,$6,$6,$7,$8,'active')
		RETURNING id
	`, []any{o.UserID, o.ID, applied.CampaignID, applied.PaymentAmount, applied.BaseCreditedAmount,
		applied.BonusAmount, applied.Currency, expiresAt}, &grantID); err != nil {
		return err
	}

	balanceAfter := balanceBefore
	if !alreadyCredited {
		if err := ScanBalanceRow(ctx, exec, `
			UPDATE users SET balance = balance + $1, updated_at = NOW()
			WHERE id = $2 AND deleted_at IS NULL RETURNING balance
		`, []any{applied.BonusAmount, o.UserID}, &balanceAfter); err != nil {
			return err
		}
	}
	bonusBefore := 0.0
	bonusAfter, err := ActiveRechargeBonusTotal(ctx, exec, o.UserID)
	if err != nil {
		return err
	}
	bonusBefore = math.Max(0, bonusAfter-applied.BonusAmount)
	if err := RecordBalanceLedger(ctx, exec, BalanceLedgerEntry{
		UserID: o.UserID, EventType: "recharge_bonus_granted", Amount: balanceAfter - balanceBefore,
		BalanceBefore: balanceBefore, BalanceAfter: balanceAfter,
		BonusBefore: bonusBefore, BonusAfter: bonusAfter,
		SourceType: "payment_order", SourceID: fmt.Sprintf("%d", o.ID),
		Description: "充值活动赠送额度",
		Metadata:    map[string]any{"grant_id": grantID, "expires_at": expiresAt, "campaign_id": applied.CampaignID},
	}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if s.redeemService != nil {
		s.redeemService.invalidateRedeemCaches(ctx, o.UserID, &RedeemCode{Type: RedeemTypeBalance})
	}
	return nil
}

// finalizeRechargeRefundAccounting mirrors a successful balance refund into the
// expiring-bonus lots and the immutable balance ledger. The balance itself was
// already deducted by the existing refund flow.
func finalizeRechargeRefundAccounting(ctx context.Context, exec BalanceSQLExecutor, p *RefundPlan) error {
	if p == nil || p.Order == nil || p.DeductionType != "balance" || p.BalanceToDeduct <= balanceEpsilon {
		return nil
	}
	var balanceAfter float64
	if err := ScanBalanceRow(ctx, exec, `SELECT balance FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, []any{p.Order.UserID}, &balanceAfter); err != nil {
		return err
	}
	bonusBefore, err := ActiveRechargeBonusTotal(ctx, exec, p.Order.UserID)
	if err != nil {
		return err
	}
	bonusAfter := bonusBefore
	var grantID int64
	var granted, remaining float64
	err = ScanBalanceRow(ctx, exec, `
		SELECT id,granted_amount,remaining_amount
		FROM recharge_bonus_grants WHERE payment_order_id=$1 FOR UPDATE
	`, []any{p.OrderID}, &grantID, &granted, &remaining)
	if err == nil && p.Order.Amount > balanceEpsilon {
		revoke := math.Min(remaining, granted*math.Min(1, p.RefundAmount/p.Order.Amount))
		if revoke > balanceEpsilon {
			newRemaining := math.Max(0, remaining-revoke)
			status := "active"
			if newRemaining <= balanceEpsilon {
				status = "revoked"
			}
			if _, err = exec.ExecContext(ctx, `UPDATE recharge_bonus_grants SET remaining_amount=$1,status=$2,updated_at=NOW() WHERE id=$3`, newRemaining, status, grantID); err != nil {
				return err
			}
			bonusAfter = math.Max(0, bonusBefore-revoke)
		}
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return RecordBalanceLedger(ctx, exec, BalanceLedgerEntry{
		UserID: p.Order.UserID, EventType: "payment_refund", Amount: -p.BalanceToDeduct,
		BalanceBefore: balanceAfter + p.BalanceToDeduct, BalanceAfter: balanceAfter,
		BonusBefore: bonusBefore, BonusAfter: bonusAfter,
		SourceType: "payment_order", SourceID: fmt.Sprintf("refund:%d", p.OrderID),
		Description: "充值退款扣回余额",
		Metadata:    map[string]any{"refund_amount": p.RefundAmount, "grant_id": grantID},
	})
}
