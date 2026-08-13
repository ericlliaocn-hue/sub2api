package repository

import (
	"context"
	"database/sql"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"time"
)

type promotionRepository struct{ db *sql.DB }

func NewPromotionRepository(db *sql.DB) service.PromotionRepository {
	return &promotionRepository{db: db}
}
func (r *promotionRepository) ListPromoters(ctx context.Context) ([]service.PromotionPromoter, error) {
	rows, e := r.db.QueryContext(ctx, `SELECT id,name,contact,commission_rate,enabled,notes,created_at,updated_at FROM promotion_promoters ORDER BY id DESC`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []service.PromotionPromoter{}
	for rows.Next() {
		var x service.PromotionPromoter
		if e = rows.Scan(&x.ID, &x.Name, &x.Contact, &x.CommissionRate, &x.Enabled, &x.Notes, &x.CreatedAt, &x.UpdatedAt); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *promotionRepository) CreatePromoter(ctx context.Context, in service.PromotionPromoterInput) (*service.PromotionPromoter, error) {
	var x service.PromotionPromoter
	e := r.db.QueryRowContext(ctx, `INSERT INTO promotion_promoters(name,contact,commission_rate,enabled,notes) VALUES($1,$2,$3,$4,$5) RETURNING id,name,contact,commission_rate,enabled,notes,created_at,updated_at`, in.Name, in.Contact, in.CommissionRate, in.Enabled, in.Notes).Scan(&x.ID, &x.Name, &x.Contact, &x.CommissionRate, &x.Enabled, &x.Notes, &x.CreatedAt, &x.UpdatedAt)
	return &x, e
}
func (r *promotionRepository) UpdatePromoter(ctx context.Context, id int64, in service.PromotionPromoterInput) (*service.PromotionPromoter, error) {
	var x service.PromotionPromoter
	e := r.db.QueryRowContext(ctx, `UPDATE promotion_promoters SET name=$2,contact=$3,commission_rate=$4,enabled=$5,notes=$6,updated_at=NOW() WHERE id=$1 RETURNING id,name,contact,commission_rate,enabled,notes,created_at,updated_at`, id, in.Name, in.Contact, in.CommissionRate, in.Enabled, in.Notes).Scan(&x.ID, &x.Name, &x.Contact, &x.CommissionRate, &x.Enabled, &x.Notes, &x.CreatedAt, &x.UpdatedAt)
	return &x, e
}
func (r *promotionRepository) ListChannels(ctx context.Context) ([]service.PromotionChannel, error) {
	rows, e := r.db.QueryContext(ctx, `SELECT c.id,c.code,c.name,c.channel_type,c.promoter_id,COALESCE(p.name,''),c.enabled,c.notes,c.created_at,c.updated_at FROM promotion_channels c LEFT JOIN promotion_promoters p ON p.id=c.promoter_id ORDER BY c.id DESC`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []service.PromotionChannel{}
	for rows.Next() {
		var x service.PromotionChannel
		var pid sql.NullInt64
		if e = rows.Scan(&x.ID, &x.Code, &x.Name, &x.ChannelType, &pid, &x.PromoterName, &x.Enabled, &x.Notes, &x.CreatedAt, &x.UpdatedAt); e != nil {
			return nil, e
		}
		if pid.Valid {
			x.PromoterID = &pid.Int64
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *promotionRepository) CreateChannel(ctx context.Context, in service.PromotionChannelInput) (*service.PromotionChannel, error) {
	var id int64
	e := r.db.QueryRowContext(ctx, `INSERT INTO promotion_channels(code,name,channel_type,promoter_id,enabled,notes) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, in.Code, in.Name, in.ChannelType, in.PromoterID, in.Enabled, in.Notes).Scan(&id)
	if e != nil {
		return nil, e
	}
	items, e := r.ListChannels(ctx)
	if e != nil {
		return nil, e
	}
	for _, x := range items {
		if x.ID == id {
			return &x, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (r *promotionRepository) UpdateChannel(ctx context.Context, id int64, in service.PromotionChannelInput) (*service.PromotionChannel, error) {
	_, e := r.db.ExecContext(ctx, `UPDATE promotion_channels SET code=$2,name=$3,channel_type=$4,promoter_id=$5,enabled=$6,notes=$7,updated_at=NOW() WHERE id=$1`, id, in.Code, in.Name, in.ChannelType, in.PromoterID, in.Enabled, in.Notes)
	if e != nil {
		return nil, e
	}
	items, e := r.ListChannels(ctx)
	if e != nil {
		return nil, e
	}
	for _, x := range items {
		if x.ID == id {
			return &x, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (r *promotionRepository) AttributeUser(ctx context.Context, userID int64, code string) error {
	_, e := r.db.ExecContext(ctx, `INSERT INTO promotion_user_attributions(user_id,channel_id) SELECT $1,id FROM promotion_channels WHERE code=$2 AND enabled=TRUE ON CONFLICT(user_id) DO NOTHING`, userID, code)
	return e
}
func (r *promotionRepository) Report(ctx context.Context, start, end time.Time) (*service.PromotionReport, error) {
	rows, e := r.db.QueryContext(ctx, `WITH cohort AS (
		SELECT c.id, c.code, c.name, c.channel_type, COALESCE(p.name,'') AS promoter_name, a.user_id,
		       u.created_at, a.attributed_at
		FROM promotion_channels c
		LEFT JOIN promotion_promoters p ON p.id=c.promoter_id
		LEFT JOIN promotion_user_attributions a ON a.channel_id=c.id
		LEFT JOIN users u ON u.id=a.user_id
	), payments AS (
		SELECT user_id, SUM(amount)::double precision AS recharge
		FROM payment_orders
		WHERE status='COMPLETED' AND COALESCE(paid_at,completed_at,created_at) >= $1 AND COALESCE(paid_at,completed_at,created_at) < $2
		GROUP BY user_id
	), usage AS (
		SELECT user_id, SUM(actual_cost)::double precision AS revenue
		FROM usage_logs WHERE created_at >= $1 AND created_at < $2 GROUP BY user_id
	), costs AS (
		SELECT CASE WHEN COALESCE(scope->>'channel_id','') ~ '^[0-9]+$' THEN (scope->>'channel_id')::bigint ELSE 0 END AS channel_id,
		       SUM(amount * exchange_rate_to_billing_unit)::double precision AS marketing_cost
		FROM business_expenses
		WHERE status='active' AND category='marketing'
		  AND COALESCE(period_end, occurred_at + interval '1 microsecond') > $1
		  AND COALESCE(period_start, occurred_at) < $2
		GROUP BY CASE WHEN COALESCE(scope->>'channel_id','') ~ '^[0-9]+$' THEN (scope->>'channel_id')::bigint ELSE 0 END
	)
	SELECT co.id,co.code,co.name,co.channel_type,co.promoter_name,
		COUNT(DISTINCT CASE WHEN co.created_at >= $1 AND co.created_at < $2 THEN co.user_id END),
		COUNT(DISTINCT pay.user_id), COUNT(DISTINCT use.user_id), COALESCE(SUM(pay.recharge),0), COALESCE(SUM(use.revenue),0), COALESCE(MAX(costs.marketing_cost),0)
	FROM cohort co LEFT JOIN payments pay ON pay.user_id=co.user_id LEFT JOIN usage use ON use.user_id=co.user_id LEFT JOIN costs ON costs.channel_id=co.id
	GROUP BY co.id,co.code,co.name,co.channel_type,co.promoter_name ORDER BY co.id`, start, end)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := &service.PromotionReport{StartTime: start, EndTime: end, Rows: []service.PromotionReportRow{}}
	for rows.Next() {
		var x service.PromotionReportRow
		if e = rows.Scan(&x.ChannelID, &x.Code, &x.Name, &x.ChannelType, &x.PromoterName, &x.NewUsers, &x.PayingUsers, &x.ActiveUsers, &x.Recharge, &x.Revenue, &x.MarketingCost); e != nil {
			return nil, e
		}
		x.Profit = x.Revenue - x.MarketingCost
		if x.PayingUsers > 0 {
			x.CAC = x.MarketingCost / float64(x.PayingUsers)
		}
		if x.MarketingCost > 0 {
			x.ROI = x.Profit / x.MarketingCost
		}
		out.Rows = append(out.Rows, x)
	}
	return out, rows.Err()
}
