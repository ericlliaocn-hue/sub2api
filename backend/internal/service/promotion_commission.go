package service

import (
	"context"
	"fmt"
	"math"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

// applyPromotionCommissionForOrder snapshots the attribution, promoter and
// effective rate once per completed order. The database unique constraint on
// payment_order_id makes fulfillment retries idempotent.
func (s *PaymentService) applyPromotionCommissionForOrder(ctx context.Context, order *dbent.PaymentOrder) error {
	if s == nil || s.entClient == nil || order == nil {
		return nil
	}
	baseAmount := paymentOrderBaseCreditedAmount(order)
	if baseAmount <= 0 || math.IsNaN(baseAmount) || math.IsInf(baseAmount, 0) {
		return nil
	}
	// PaymentOrder.Amount is the credited billing-unit amount (including any
	// bonus); paymentOrderBaseCreditedAmount removes the bonus portion.
	currency := "BILLING"
	_, err := s.entClient.ExecContext(ctx, `
		INSERT INTO promotion_commission_ledger (
			payment_order_id,user_id,channel_id,promoter_id,base_amount,commission_rate,amount,
			currency,status,frozen_until,channel_code_snapshot,channel_name_snapshot,promoter_name_snapshot
		)
		SELECT $1,$2,c.id,p.id,$3,
		       COALESCE(c.commission_rate,p.commission_rate),
		       ROUND(($3::numeric*COALESCE(c.commission_rate,p.commission_rate)/100),8),
		       $4,
		       CASE WHEN p.commission_freeze_days>0 THEN 'frozen' ELSE 'available' END,
		       CASE WHEN p.commission_freeze_days>0 THEN NOW()+(p.commission_freeze_days*INTERVAL '1 day') ELSE NULL END,
		       c.code,c.name,p.name
		FROM promotion_user_attributions a
		JOIN promotion_channels c ON c.id=a.channel_id AND c.enabled=TRUE
		JOIN promotion_promoters p ON p.id=c.promoter_id AND p.enabled=TRUE
		WHERE a.user_id=$2 AND COALESCE(c.commission_rate,p.commission_rate)>0
		ON CONFLICT(payment_order_id) DO NOTHING`, order.ID, order.UserID, baseAmount, currency)
	if err != nil {
		return fmt.Errorf("accrue promotion commission: %w", err)
	}
	return nil
}

// reversePromotionCommission applies the same refund ratio used by the order.
// A settled entry remains linked to its settlement so the recovery remains
// auditable; reports use amount-reversed_amount as the net commission cost.
func reversePromotionCommission(ctx context.Context, exec BalanceSQLExecutor, plan *RefundPlan) error {
	if exec == nil || plan == nil || plan.Order == nil || plan.Order.Amount <= 0 || plan.RefundAmount <= 0 {
		return nil
	}
	ratio := math.Min(1, plan.RefundAmount/plan.Order.Amount)
	_, err := exec.ExecContext(ctx, `
		UPDATE promotion_commission_ledger
		SET reversed_amount=LEAST(amount,reversed_amount+ROUND(amount*$2::numeric,8)),
		    reversed_at=NOW(),
		    status=CASE
		      WHEN settlement_id IS NULL AND reversed_amount+ROUND(amount*$2::numeric,8)>=amount THEN 'reversed'
		      ELSE status
		    END,
		    updated_at=NOW()
		WHERE payment_order_id=$1`, plan.OrderID, ratio)
	if err != nil {
		return fmt.Errorf("reverse promotion commission: %w", err)
	}
	return nil
}
