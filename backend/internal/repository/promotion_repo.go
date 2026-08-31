package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type promotionRepository struct{ db *sql.DB }

func NewPromotionRepository(db *sql.DB) service.PromotionRepository {
	return &promotionRepository{db: db}
}

func (r *promotionRepository) ListPromoters(ctx context.Context) ([]service.PromotionPromoter, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,contact,commission_rate,commission_freeze_days,enabled,notes,created_at,updated_at FROM promotion_promoters ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.PromotionPromoter{}
	for rows.Next() {
		var x service.PromotionPromoter
		if err = rows.Scan(&x.ID, &x.Name, &x.Contact, &x.CommissionRate, &x.CommissionFreezeDays, &x.Enabled, &x.Notes, &x.CreatedAt, &x.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (r *promotionRepository) CreatePromoter(ctx context.Context, in service.PromotionPromoterInput) (*service.PromotionPromoter, error) {
	var x service.PromotionPromoter
	err := r.db.QueryRowContext(ctx, `INSERT INTO promotion_promoters(name,contact,commission_rate,commission_freeze_days,enabled,notes) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,name,contact,commission_rate,commission_freeze_days,enabled,notes,created_at,updated_at`, in.Name, in.Contact, in.CommissionRate, in.CommissionFreezeDays, in.Enabled, in.Notes).Scan(&x.ID, &x.Name, &x.Contact, &x.CommissionRate, &x.CommissionFreezeDays, &x.Enabled, &x.Notes, &x.CreatedAt, &x.UpdatedAt)
	return &x, err
}

func (r *promotionRepository) UpdatePromoter(ctx context.Context, id int64, in service.PromotionPromoterInput) (*service.PromotionPromoter, error) {
	var x service.PromotionPromoter
	err := r.db.QueryRowContext(ctx, `UPDATE promotion_promoters SET name=$2,contact=$3,commission_rate=$4,commission_freeze_days=$5,enabled=$6,notes=$7,updated_at=NOW() WHERE id=$1 RETURNING id,name,contact,commission_rate,commission_freeze_days,enabled,notes,created_at,updated_at`, id, in.Name, in.Contact, in.CommissionRate, in.CommissionFreezeDays, in.Enabled, in.Notes).Scan(&x.ID, &x.Name, &x.Contact, &x.CommissionRate, &x.CommissionFreezeDays, &x.Enabled, &x.Notes, &x.CreatedAt, &x.UpdatedAt)
	return &x, err
}

func (r *promotionRepository) ListChannels(ctx context.Context) ([]service.PromotionChannel, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT c.id,c.code,c.name,c.channel_type,c.promoter_id,COALESCE(p.name,''),c.commission_rate,c.enabled,c.notes,c.created_at,c.updated_at FROM promotion_channels c LEFT JOIN promotion_promoters p ON p.id=c.promoter_id ORDER BY c.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.PromotionChannel{}
	for rows.Next() {
		var x service.PromotionChannel
		var promoterID sql.NullInt64
		var rate sql.NullFloat64
		if err = rows.Scan(&x.ID, &x.Code, &x.Name, &x.ChannelType, &promoterID, &x.PromoterName, &rate, &x.Enabled, &x.Notes, &x.CreatedAt, &x.UpdatedAt); err != nil {
			return nil, err
		}
		if promoterID.Valid {
			x.PromoterID = &promoterID.Int64
		}
		if rate.Valid {
			x.CommissionRate = &rate.Float64
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (r *promotionRepository) CreateChannel(ctx context.Context, in service.PromotionChannelInput) (*service.PromotionChannel, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `INSERT INTO promotion_channels(code,name,channel_type,promoter_id,commission_rate,enabled,notes) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, in.Code, in.Name, in.ChannelType, in.PromoterID, in.CommissionRate, in.Enabled, in.Notes).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.getChannel(ctx, id)
}

func (r *promotionRepository) UpdateChannel(ctx context.Context, id int64, in service.PromotionChannelInput) (*service.PromotionChannel, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE promotion_channels SET code=$2,name=$3,channel_type=$4,promoter_id=$5,commission_rate=$6,enabled=$7,notes=$8,updated_at=NOW() WHERE id=$1`, id, in.Code, in.Name, in.ChannelType, in.PromoterID, in.CommissionRate, in.Enabled, in.Notes)
	if err != nil {
		return nil, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil, sql.ErrNoRows
	}
	return r.getChannel(ctx, id)
}

func (r *promotionRepository) getChannel(ctx context.Context, id int64) (*service.PromotionChannel, error) {
	var x service.PromotionChannel
	var promoterID sql.NullInt64
	var rate sql.NullFloat64
	err := r.db.QueryRowContext(ctx, `SELECT c.id,c.code,c.name,c.channel_type,c.promoter_id,COALESCE(p.name,''),c.commission_rate,c.enabled,c.notes,c.created_at,c.updated_at FROM promotion_channels c LEFT JOIN promotion_promoters p ON p.id=c.promoter_id WHERE c.id=$1`, id).Scan(&x.ID, &x.Code, &x.Name, &x.ChannelType, &promoterID, &x.PromoterName, &rate, &x.Enabled, &x.Notes, &x.CreatedAt, &x.UpdatedAt)
	if promoterID.Valid {
		x.PromoterID = &promoterID.Int64
	}
	if rate.Valid {
		x.CommissionRate = &rate.Float64
	}
	return &x, err
}

func (r *promotionRepository) AttributeUser(ctx context.Context, userID int64, code string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var channelID int64
	var enabled bool
	err = tx.QueryRowContext(ctx, `SELECT id,enabled FROM promotion_channels WHERE code=$1`, code).Scan(&channelID, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `INSERT INTO promotion_attribution_events(user_id,requested_code,outcome,detail) VALUES($1,$2,'invalid_code','channel code does not exist')`, userID, code)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	if !enabled {
		_, err = tx.ExecContext(ctx, `INSERT INTO promotion_attribution_events(user_id,requested_code,channel_id,outcome,detail) VALUES($1,$2,$3,'channel_disabled','channel is disabled')`, userID, code, channelID)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO promotion_user_attributions(user_id,channel_id) VALUES($1,$2) ON CONFLICT(user_id) DO NOTHING`, userID, channelID)
	if err != nil {
		return err
	}
	outcome, detail := "attributed", "first-touch attribution recorded"
	if affected, _ := result.RowsAffected(); affected == 0 {
		outcome, detail = "already_attributed", "existing first-touch attribution retained"
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO promotion_attribution_events(user_id,requested_code,channel_id,outcome,detail) VALUES($1,$2,$3,$4,$5)`, userID, code, channelID, outcome, detail)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *promotionRepository) ListAttributionEvents(ctx context.Context, limit int) ([]service.PromotionAttributionEvent, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT e.id,e.user_id,COALESCE(u.email,''),e.requested_code,e.channel_id,COALESCE(c.name,''),e.outcome,e.detail,e.created_at FROM promotion_attribution_events e JOIN users u ON u.id=e.user_id LEFT JOIN promotion_channels c ON c.id=e.channel_id ORDER BY e.created_at DESC,e.id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.PromotionAttributionEvent{}
	for rows.Next() {
		var x service.PromotionAttributionEvent
		var channelID sql.NullInt64
		if err = rows.Scan(&x.ID, &x.UserID, &x.UserEmail, &x.RequestedCode, &channelID, &x.ChannelName, &x.Outcome, &x.Detail, &x.CreatedAt); err != nil {
			return nil, err
		}
		if channelID.Valid {
			x.ChannelID = &channelID.Int64
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (r *promotionRepository) ListCommissions(ctx context.Context, promoterID int64, status string, limit int) ([]service.PromotionCommission, error) {
	if _, err := r.db.ExecContext(ctx, `UPDATE promotion_commission_ledger SET status='available',updated_at=NOW() WHERE status='frozen' AND frozen_until<=NOW()`); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT l.id,l.payment_order_id,l.user_id,COALESCE(u.email,''),l.channel_id,l.channel_code_snapshot,l.channel_name_snapshot,l.promoter_id,l.promoter_name_snapshot,l.base_amount::double precision,l.commission_rate::double precision,l.amount::double precision,l.reversed_amount::double precision,l.currency,l.status,l.frozen_until,l.settlement_id,l.created_at FROM promotion_commission_ledger l JOIN users u ON u.id=l.user_id WHERE ($1=0 OR l.promoter_id=$1) AND ($2='' OR l.status=$2) ORDER BY l.created_at DESC,l.id DESC LIMIT $3`, promoterID, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.PromotionCommission{}
	for rows.Next() {
		var x service.PromotionCommission
		var frozen sql.NullTime
		var settlement sql.NullInt64
		if err = rows.Scan(&x.ID, &x.PaymentOrderID, &x.UserID, &x.UserEmail, &x.ChannelID, &x.ChannelCode, &x.ChannelName, &x.PromoterID, &x.PromoterName, &x.BaseAmount, &x.CommissionRate, &x.Amount, &x.ReversedAmount, &x.Currency, &x.Status, &frozen, &settlement, &x.CreatedAt); err != nil {
			return nil, err
		}
		if frozen.Valid {
			x.FrozenUntil = &frozen.Time
		}
		if settlement.Valid {
			x.SettlementID = &settlement.Int64
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (r *promotionRepository) ListSettlements(ctx context.Context, promoterID int64, limit int) ([]service.PromotionSettlement, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT s.id,s.promoter_id,p.name,s.period_end,s.amount::double precision,s.status,s.notes,s.paid_at,s.created_at FROM promotion_commission_settlements s JOIN promotion_promoters p ON p.id=s.promoter_id WHERE ($1=0 OR s.promoter_id=$1) ORDER BY s.created_at DESC,s.id DESC LIMIT $2`, promoterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.PromotionSettlement{}
	for rows.Next() {
		var x service.PromotionSettlement
		var paid sql.NullTime
		if err = rows.Scan(&x.ID, &x.PromoterID, &x.PromoterName, &x.PeriodEnd, &x.Amount, &x.Status, &x.Notes, &paid, &x.CreatedAt); err != nil {
			return nil, err
		}
		if paid.Valid {
			x.PaidAt = &paid.Time
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (r *promotionRepository) CreateSettlement(ctx context.Context, in service.PromotionSettlementInput) (*service.PromotionSettlement, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `UPDATE promotion_commission_ledger SET status='available',updated_at=NOW() WHERE status='frozen' AND frozen_until<=NOW()`); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,(amount-reversed_amount)::double precision FROM promotion_commission_ledger WHERE promoter_id=$1 AND status='available' AND settlement_id IS NULL AND created_at<$2 AND amount>reversed_amount ORDER BY id FOR UPDATE`, in.PromoterID, in.PeriodEnd)
	if err != nil {
		return nil, err
	}
	ids := []int64{}
	amount := 0.0
	for rows.Next() {
		var id int64
		var value float64
		if err = rows.Scan(&id, &value); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
		amount += value
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	if len(ids) == 0 || amount <= 0 {
		return nil, service.ErrPromotionSettlementEmpty
	}
	var id int64
	if err = tx.QueryRowContext(ctx, `INSERT INTO promotion_commission_settlements(promoter_id,period_end,amount,notes) VALUES($1,$2,$3,$4) RETURNING id`, in.PromoterID, in.PeriodEnd, amount, in.Notes).Scan(&id); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE promotion_commission_ledger SET settlement_id=$1,status='settled',updated_at=NOW() WHERE id=ANY($2)`, id, pq.Array(ids)); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.getSettlement(ctx, id)
}

func (r *promotionRepository) UpdateSettlementStatus(ctx context.Context, id int64, status string) (*service.PromotionSettlement, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var current string
	if err = tx.QueryRowContext(ctx, `SELECT status FROM promotion_commission_settlements WHERE id=$1 FOR UPDATE`, id).Scan(&current); err != nil {
		return nil, err
	}
	if current != "draft" {
		return nil, service.ErrPromotionSettlementState
	}
	if status == "paid" {
		_, err = tx.ExecContext(ctx, `UPDATE promotion_commission_settlements SET status='paid',paid_at=NOW(),updated_at=NOW() WHERE id=$1`, id)
	} else {
		if _, err = tx.ExecContext(ctx, `UPDATE promotion_commission_ledger SET settlement_id=NULL,status=CASE WHEN frozen_until>NOW() THEN 'frozen' ELSE 'available' END,updated_at=NOW() WHERE settlement_id=$1 AND status='settled'`, id); err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE promotion_commission_settlements SET status='cancelled',updated_at=NOW() WHERE id=$1`, id)
		}
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.getSettlement(ctx, id)
}

func (r *promotionRepository) getSettlement(ctx context.Context, id int64) (*service.PromotionSettlement, error) {
	var x service.PromotionSettlement
	var paid sql.NullTime
	err := r.db.QueryRowContext(ctx, `SELECT s.id,s.promoter_id,p.name,s.period_end,s.amount::double precision,s.status,s.notes,s.paid_at,s.created_at FROM promotion_commission_settlements s JOIN promotion_promoters p ON p.id=s.promoter_id WHERE s.id=$1`, id).Scan(&x.ID, &x.PromoterID, &x.PromoterName, &x.PeriodEnd, &x.Amount, &x.Status, &x.Notes, &paid, &x.CreatedAt)
	if paid.Valid {
		x.PaidAt = &paid.Time
	}
	return &x, err
}

func (r *promotionRepository) Report(ctx context.Context, start, end time.Time, mode string) (*service.PromotionReport, error) {
	rows, err := r.db.QueryContext(ctx, reportPromotionSQL, start, end, mode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &service.PromotionReport{StartTime: start, EndTime: end, Mode: mode, Rows: []service.PromotionReportRow{}}
	for rows.Next() {
		var x service.PromotionReportRow
		if err = rows.Scan(&x.ChannelID, &x.Code, &x.Name, &x.ChannelType, &x.PromoterName, &x.NewUsers, &x.PayingUsers, &x.ActiveUsers, &x.Recharge, &x.Revenue, &x.UpstreamCost, &x.BonusCost, &x.AffiliateCost, &x.CommissionCost, &x.PaymentFee, &x.MarketingCost); err != nil {
			return nil, err
		}
		x.Profit = x.Revenue - x.UpstreamCost - x.BonusCost - x.AffiliateCost - x.CommissionCost - x.PaymentFee - x.MarketingCost
		if x.NewUsers > 0 {
			x.CAC = x.MarketingCost / float64(x.NewUsers)
		}
		if x.PayingUsers > 0 {
			x.LTV = x.Revenue / float64(x.PayingUsers)
		}
		if x.MarketingCost > 0 {
			x.ROI = x.Profit / x.MarketingCost
		}
		out.Rows = append(out.Rows, x)
	}
	return out, rows.Err()
}

const reportPromotionSQL = `WITH attributed AS (
 SELECT a.channel_id,a.user_id,u.created_at AS user_created_at FROM promotion_user_attributions a JOIN users u ON u.id=a.user_id
), eligible AS (
 SELECT * FROM attributed WHERE $3='operation' OR (user_created_at >= $1 AND user_created_at < $2)
), registrations AS (
 SELECT channel_id,COUNT(*)::bigint AS new_users FROM attributed WHERE user_created_at >= $1 AND user_created_at < $2 GROUP BY channel_id
), payments AS (
 SELECT e.channel_id,po.id,po.user_id,GREATEST(po.amount-COALESCE(po.refund_amount,0),0)::double precision AS amount,
        (GREATEST(po.amount-COALESCE(po.refund_amount,0),0)*po.fee_rate/100)::double precision AS payment_fee
 FROM eligible e JOIN payment_orders po ON po.user_id=e.user_id
 WHERE po.status IN ('COMPLETED','PARTIALLY_REFUNDED')
   AND COALESCE(po.paid_at,po.completed_at,po.created_at) >= CASE WHEN $3='acquisition' THEN e.user_created_at ELSE $1 END
   AND COALESCE(po.paid_at,po.completed_at,po.created_at) < $2
), payment_stats AS (
 SELECT channel_id,COUNT(DISTINCT user_id)::bigint AS paying_users,COALESCE(SUM(amount),0)::double precision AS recharge,COALESCE(SUM(payment_fee),0)::double precision AS payment_fee FROM payments GROUP BY channel_id
), usage_stats AS (
 SELECT e.channel_id,COUNT(DISTINCT ul.user_id)::bigint AS active_users,COALESCE(SUM(ul.actual_cost),0)::double precision AS revenue,
 COALESCE(SUM(COALESCE(ul.upstream_cost,COALESCE(ul.account_stats_cost,ul.total_cost)*COALESCE(ul.account_rate_multiplier,1))),0)::double precision AS upstream_cost
 FROM eligible e JOIN usage_logs ul ON ul.user_id=e.user_id
 WHERE ul.created_at >= CASE WHEN $3='acquisition' THEN e.user_created_at ELSE $1 END AND ul.created_at < $2 GROUP BY e.channel_id
), bonus_stats AS (
 SELECT p.channel_id,COALESCE(SUM(g.granted_amount),0)::double precision AS bonus_cost FROM payments p JOIN recharge_bonus_grants g ON g.payment_order_id=p.id GROUP BY p.channel_id
), affiliate_stats AS (
 SELECT p.channel_id,COALESCE(SUM(l.amount),0)::double precision AS affiliate_cost FROM payments p JOIN user_affiliate_ledger l ON l.source_order_id=p.id AND l.action='accrue' GROUP BY p.channel_id
), commission_stats AS (
 SELECT p.channel_id,COALESCE(SUM(GREATEST(l.amount-l.reversed_amount,0)),0)::double precision AS commission_cost FROM payments p JOIN promotion_commission_ledger l ON l.payment_order_id=p.id GROUP BY p.channel_id
), marketing AS (
 SELECT CASE WHEN COALESCE(scope->>'channel_id','') ~ '^[0-9]+$' THEN (scope->>'channel_id')::bigint ELSE 0 END AS channel_id,COALESCE(SUM(amount*exchange_rate_to_billing_unit),0)::double precision AS marketing_cost
 FROM business_expenses WHERE status='active' AND category='marketing' AND COALESCE(period_end,occurred_at+interval '1 microsecond')>$1 AND COALESCE(period_start,occurred_at)<$2
 GROUP BY CASE WHEN COALESCE(scope->>'channel_id','') ~ '^[0-9]+$' THEN (scope->>'channel_id')::bigint ELSE 0 END
)
SELECT c.id,c.code,c.name,c.channel_type,COALESCE(pr.name,''),COALESCE(reg.new_users,0),COALESCE(pay.paying_users,0),COALESCE(us.active_users,0),COALESCE(pay.recharge,0),COALESCE(us.revenue,0),COALESCE(us.upstream_cost,0),COALESCE(bs.bonus_cost,0),COALESCE(afs.affiliate_cost,0),COALESCE(cs.commission_cost,0),COALESCE(pay.payment_fee,0),COALESCE(m.marketing_cost,0)
FROM promotion_channels c LEFT JOIN promotion_promoters pr ON pr.id=c.promoter_id LEFT JOIN registrations reg ON reg.channel_id=c.id LEFT JOIN payment_stats pay ON pay.channel_id=c.id LEFT JOIN usage_stats us ON us.channel_id=c.id LEFT JOIN bonus_stats bs ON bs.channel_id=c.id LEFT JOIN affiliate_stats afs ON afs.channel_id=c.id LEFT JOIN commission_stats cs ON cs.channel_id=c.id LEFT JOIN marketing m ON m.channel_id=c.id ORDER BY c.id`
