package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
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

const promotionChannelSelect = `SELECT c.id,c.code,c.name,c.channel_type,c.promoter_id,COALESCE(p.name,''),c.commission_rate,c.enabled,c.system,c.notes,c.created_at,c.updated_at FROM promotion_channels c LEFT JOIN promotion_promoters p ON p.id=c.promoter_id`

func (r *promotionRepository) ListChannels(ctx context.Context) ([]service.PromotionChannel, error) {
	rows, err := r.db.QueryContext(ctx, promotionChannelSelect+` ORDER BY c.system DESC, c.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.PromotionChannel{}
	for rows.Next() {
		var x service.PromotionChannel
		var promoterID sql.NullInt64
		var rate sql.NullFloat64
		if err = rows.Scan(&x.ID, &x.Code, &x.Name, &x.ChannelType, &promoterID, &x.PromoterName, &rate, &x.Enabled, &x.System, &x.Notes, &x.CreatedAt, &x.UpdatedAt); err != nil {
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
	existing, err := r.getChannel(ctx, id)
	if err != nil {
		return nil, err
	}
	var result sql.Result
	if existing.System {
		// 系统渠道：编码、类型、负责人、佣金、启用状态都锁死，只能改名称和备注。
		result, err = r.db.ExecContext(ctx, `UPDATE promotion_channels SET name=$2,notes=$3,updated_at=NOW() WHERE id=$1 AND system=TRUE`, id, in.Name, in.Notes)
	} else {
		result, err = r.db.ExecContext(ctx, `UPDATE promotion_channels SET code=$2,name=$3,channel_type=$4,promoter_id=$5,commission_rate=$6,enabled=$7,notes=$8,updated_at=NOW() WHERE id=$1 AND system=FALSE`, id, in.Code, in.Name, in.ChannelType, in.PromoterID, in.CommissionRate, in.Enabled, in.Notes)
	}
	if err != nil {
		return nil, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil, sql.ErrNoRows
	}
	return r.getChannel(ctx, id)
}

func (r *promotionRepository) getChannel(ctx context.Context, id int64) (*service.PromotionChannel, error) {
	return r.scanChannel(r.db.QueryRowContext(ctx, promotionChannelSelect+` WHERE c.id=$1`, id))
}

func (r *promotionRepository) GetChannelByCode(ctx context.Context, code string) (*service.PromotionChannel, error) {
	return r.scanChannel(r.db.QueryRowContext(ctx, promotionChannelSelect+` WHERE c.code=$1`, code))
}

func (r *promotionRepository) scanChannel(row *sql.Row) (*service.PromotionChannel, error) {
	var x service.PromotionChannel
	var promoterID sql.NullInt64
	var rate sql.NullFloat64
	err := row.Scan(&x.ID, &x.Code, &x.Name, &x.ChannelType, &promoterID, &x.PromoterName, &rate, &x.Enabled, &x.System, &x.Notes, &x.CreatedAt, &x.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if promoterID.Valid {
		x.PromoterID = &promoterID.Int64
	}
	if rate.Valid {
		x.CommissionRate = &rate.Float64
	}
	return &x, nil
}

func (r *promotionRepository) AttributeUser(ctx context.Context, in service.PromotionAttributionInput) (bool, error) {
	utm, err := json.Marshal(nonNilStringMap(in.UTM))
	if err != nil {
		return false, err
	}
	evidence, err := json.Marshal(nonNilAnyMap(in.Evidence))
	if err != nil {
		return false, err
	}
	firstTouch := in.FirstTouchAt
	if firstTouch.IsZero() {
		firstTouch = time.Now()
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO promotion_user_attributions(user_id,channel_id,acquisition_class,first_seen_at,landing_path,referrer,utm,evidence)
SELECT $1,c.id,$3,$4,$5,$6,$7::jsonb,$8::jsonb FROM promotion_channels c WHERE c.code=$2
ON CONFLICT(user_id) DO NOTHING`, in.UserID, in.ChannelCode, in.Class, firstTouch, in.LandingPath, in.ReferrerHost, string(utm), string(evidence))
	if err != nil {
		return false, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		// 区分“已归因”与“渠道编码不存在”（系统渠道被删才会发生）。
		var exists bool
		if scanErr := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM promotion_user_attributions WHERE user_id=$1)`, in.UserID).Scan(&exists); scanErr != nil {
			return false, scanErr
		}
		if !exists {
			return false, fmt.Errorf("promotion channel %q not found", in.ChannelCode)
		}
		return false, nil
	}
	return true, nil
}

func (r *promotionRepository) RecordAttributionEvent(ctx context.Context, in service.PromotionAttributionEventInput) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO promotion_attribution_events(user_id,requested_code,channel_id,acquisition_class,outcome,detail) VALUES($1,$2,$3,$4,$5,$6)`, in.UserID, in.RequestedCode, in.ChannelID, in.Class, in.Outcome, in.Detail)
	return err
}

func (r *promotionRepository) RecordVisit(ctx context.Context, in service.AcquisitionVisitInput) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO acquisition_visits_daily(day,acquisition_class,channel_id,engine,referrer_host,landing_path,visits,updated_at)
SELECT $1::date,$2,COALESCE((SELECT id FROM promotion_channels WHERE code=$3),0),$4,$5,$6,1,NOW()
ON CONFLICT(day,acquisition_class,channel_id,engine,referrer_host,landing_path) DO UPDATE SET visits=acquisition_visits_daily.visits+1,updated_at=NOW()`,
		in.Day.In(timezone.Location()).Format("2006-01-02"), in.Class, in.ChannelCode, in.Engine, in.ReferrerHost, in.LandingPath)
	return err
}

func nonNilStringMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func nonNilAnyMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

func (r *promotionRepository) ListAttributionEvents(ctx context.Context, limit int) ([]service.PromotionAttributionEvent, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT e.id,e.user_id,COALESCE(u.email,''),e.requested_code,e.channel_id,COALESCE(c.name,''),e.acquisition_class,e.outcome,e.detail,e.created_at FROM promotion_attribution_events e JOIN users u ON u.id=e.user_id LEFT JOIN promotion_channels c ON c.id=e.channel_id ORDER BY e.created_at DESC,e.id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.PromotionAttributionEvent{}
	for rows.Next() {
		var x service.PromotionAttributionEvent
		var channelID sql.NullInt64
		if err = rows.Scan(&x.ID, &x.UserID, &x.UserEmail, &x.RequestedCode, &channelID, &x.ChannelName, &x.AcquisitionClass, &x.Outcome, &x.Detail, &x.CreatedAt); err != nil {
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
	out := &service.PromotionReport{
		StartTime: start, EndTime: end, Mode: mode,
		Rows:            []service.PromotionReportRow{},
		Classes:         []service.PromotionReportClassRow{},
		SEOEngines:      []service.PromotionBreakdownRow{},
		SEOLandingPages: []service.PromotionBreakdownRow{},
		Inviters:        []service.PromotionBreakdownRow{},
		ExternalHosts:   []service.PromotionBreakdownRow{},
	}
	startDay, endDay := acquisitionDayRange(start, end)

	rows, err := r.db.QueryContext(ctx, reportPromotionSQL, start, end, mode, startDay, endDay)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var x service.PromotionReportRow
		if err = rows.Scan(&x.ChannelID, &x.Code, &x.Name, &x.ChannelType, &x.System, &x.PromoterName, &x.Visits, &x.NewUsers, &x.InvitedUsers, &x.PayingUsers, &x.ActiveUsers, &x.Recharge, &x.Revenue, &x.UpstreamCost, &x.BonusCost, &x.AffiliateCost, &x.CommissionCost, &x.PaymentFee, &x.MarketingCost); err != nil {
			return nil, err
		}
		x.AcquisitionClass = x.ChannelType
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
		if x.Visits > 0 {
			x.ConversionRate = float64(x.NewUsers) / float64(x.Visits)
		}
		out.Rows = append(out.Rows, x)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	aggregatePromotionClasses(out)

	// 区间实际注册人数：与四分类之和不一致就是归因漏人。
	if err = r.db.QueryRowContext(ctx, `SELECT COUNT(*)::bigint FROM users WHERE created_at >= $1 AND created_at < $2`, start, end).Scan(&out.Totals.RegisteredUsers); err != nil {
		return nil, err
	}
	out.Totals.UnattributedUsers = out.Totals.RegisteredUsers - out.Totals.NewUsers
	if out.Totals.UnattributedUsers < 0 {
		out.Totals.UnattributedUsers = 0
	}

	if out.SEOEngines, err = r.queryBreakdown(ctx, reportSEOEngineSQL, start, end, startDay, endDay); err != nil {
		return nil, err
	}
	if out.SEOLandingPages, err = r.queryBreakdown(ctx, reportSEOLandingSQL, start, end, startDay, endDay); err != nil {
		return nil, err
	}
	if out.Inviters, err = r.queryBreakdown(ctx, reportInviterSQL, start, end, startDay, endDay); err != nil {
		return nil, err
	}
	if out.ExternalHosts, err = r.queryBreakdown(ctx, reportExternalHostSQL, start, end, startDay, endDay); err != nil {
		return nil, err
	}
	return out, nil
}

// acquisitionDayRange 把区间换成系统时区的日期边界，供 acquisition_visits_daily 使用。
func acquisitionDayRange(start, end time.Time) (string, string) {
	loc := timezone.Location()
	startDay := start.In(loc).Format("2006-01-02")
	// end 是开区间；落在当天 00:00 精确边界时不多算一天。
	endLocal := end.In(loc)
	endDay := endLocal.Format("2006-01-02")
	if !(endLocal.Hour() == 0 && endLocal.Minute() == 0 && endLocal.Second() == 0 && endLocal.Nanosecond() == 0) {
		endDay = endLocal.AddDate(0, 0, 1).Format("2006-01-02")
	}
	return startDay, endDay
}

func aggregatePromotionClasses(out *service.PromotionReport) {
	byClass := map[string]*service.PromotionReportClassRow{}
	for _, class := range service.AcquisitionClassOrder {
		byClass[class] = &service.PromotionReportClassRow{Class: class}
	}
	for _, row := range out.Rows {
		target, ok := byClass[row.ChannelType]
		if !ok {
			target = byClass[service.AcquisitionClassOther]
		}
		target.Visits += row.Visits
		target.NewUsers += row.NewUsers
		target.InvitedUsers += row.InvitedUsers
		target.PayingUsers += row.PayingUsers
		target.ActiveUsers += row.ActiveUsers
		target.Recharge += row.Recharge
		target.Revenue += row.Revenue
		target.UpstreamCost += row.UpstreamCost
		target.BonusCost += row.BonusCost
		target.AffiliateCost += row.AffiliateCost
		target.CommissionCost += row.CommissionCost
		target.PaymentFee += row.PaymentFee
		target.MarketingCost += row.MarketingCost
		target.Profit += row.Profit

		out.Totals.Visits += row.Visits
		out.Totals.NewUsers += row.NewUsers
		out.Totals.InvitedUsers += row.InvitedUsers
		out.Totals.PayingUsers += row.PayingUsers
		out.Totals.ActiveUsers += row.ActiveUsers
		out.Totals.Recharge += row.Recharge
		out.Totals.Revenue += row.Revenue
		out.Totals.Profit += row.Profit
	}
	if out.Totals.Visits > 0 {
		out.Totals.ConversionRate = float64(out.Totals.NewUsers) / float64(out.Totals.Visits)
	}
	for _, class := range service.AcquisitionClassOrder {
		x := byClass[class]
		if x.NewUsers > 0 {
			x.CAC = x.MarketingCost / float64(x.NewUsers)
		}
		if x.PayingUsers > 0 {
			x.LTV = x.Revenue / float64(x.PayingUsers)
		}
		if x.MarketingCost > 0 {
			x.ROI = x.Profit / x.MarketingCost
		}
		if x.Visits > 0 {
			x.ConversionRate = float64(x.NewUsers) / float64(x.Visits)
		}
		if out.Totals.NewUsers > 0 {
			x.NewUsersShare = float64(x.NewUsers) / float64(out.Totals.NewUsers)
		}
		out.Classes = append(out.Classes, *x)
	}
}

func (r *promotionRepository) queryBreakdown(ctx context.Context, query string, start, end time.Time, startDay, endDay string) ([]service.PromotionBreakdownRow, error) {
	rows, err := r.db.QueryContext(ctx, query, start, end, startDay, endDay)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.PromotionBreakdownRow{}
	for rows.Next() {
		var x service.PromotionBreakdownRow
		if err = rows.Scan(&x.Key, &x.Label, &x.Visits, &x.NewUsers, &x.PayingUsers, &x.Revenue, &x.Extra); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// 区间注册用户的付费 / 消耗（下钻口径固定为拉新同期，不区分 mode）。
const reportCohortCTE = `WITH cohort AS (
 SELECT a.user_id,a.channel_id,a.acquisition_class,a.landing_path,a.referrer,a.evidence,u.created_at AS user_created_at
 FROM promotion_user_attributions a JOIN users u ON u.id=a.user_id WHERE u.created_at >= $1 AND u.created_at < $2
), cohort_pay AS (
 SELECT c.user_id,COUNT(DISTINCT po.id)>0 AS paid FROM cohort c JOIN payment_orders po ON po.user_id=c.user_id AND po.status IN ('COMPLETED','PARTIALLY_REFUNDED') AND COALESCE(po.paid_at,po.completed_at,po.created_at) < $2 GROUP BY c.user_id
), cohort_rev AS (
 SELECT c.user_id,COALESCE(SUM(ul.actual_cost),0)::double precision AS revenue FROM cohort c JOIN usage_logs ul ON ul.user_id=c.user_id AND ul.created_at < $2 GROUP BY c.user_id
)`

const reportSEOEngineSQL = reportCohortCTE + `, reg AS (
 SELECT COALESCE(NULLIF(c.evidence->>'engine',''),'unknown') AS k,COUNT(*)::bigint AS new_users,COUNT(*) FILTER (WHERE cp.paid)::bigint AS paying_users,COALESCE(SUM(cr.revenue),0)::double precision AS revenue
 FROM cohort c LEFT JOIN cohort_pay cp ON cp.user_id=c.user_id LEFT JOIN cohort_rev cr ON cr.user_id=c.user_id WHERE c.acquisition_class='seo' GROUP BY 1
), vis AS (
 SELECT COALESCE(NULLIF(engine,''),'unknown') AS k,SUM(visits)::bigint AS visits FROM acquisition_visits_daily WHERE acquisition_class='seo' AND day >= $3::date AND day < $4::date GROUP BY 1
)
SELECT COALESCE(reg.k,vis.k),COALESCE(reg.k,vis.k),COALESCE(vis.visits,0),COALESCE(reg.new_users,0),COALESCE(reg.paying_users,0),COALESCE(reg.revenue,0),0::double precision
FROM reg FULL OUTER JOIN vis ON vis.k=reg.k ORDER BY COALESCE(reg.new_users,0) DESC,COALESCE(vis.visits,0) DESC LIMIT 20`

const reportSEOLandingSQL = reportCohortCTE + `, reg AS (
 SELECT COALESCE(NULLIF(c.landing_path,''),'/') AS k,COUNT(*)::bigint AS new_users,COUNT(*) FILTER (WHERE cp.paid)::bigint AS paying_users,COALESCE(SUM(cr.revenue),0)::double precision AS revenue
 FROM cohort c LEFT JOIN cohort_pay cp ON cp.user_id=c.user_id LEFT JOIN cohort_rev cr ON cr.user_id=c.user_id WHERE c.acquisition_class='seo' GROUP BY 1
), vis AS (
 SELECT COALESCE(NULLIF(landing_path,''),'/') AS k,SUM(visits)::bigint AS visits FROM acquisition_visits_daily WHERE acquisition_class='seo' AND day >= $3::date AND day < $4::date GROUP BY 1
)
SELECT COALESCE(reg.k,vis.k),COALESCE(reg.k,vis.k),COALESCE(vis.visits,0),COALESCE(reg.new_users,0),COALESCE(reg.paying_users,0),COALESCE(reg.revenue,0),0::double precision
FROM reg FULL OUTER JOIN vis ON vis.k=reg.k ORDER BY COALESCE(vis.visits,0) DESC,COALESCE(reg.new_users,0) DESC LIMIT 20`

// 邀请人 Top：带来多少注册 / 付费 / 消耗，Extra = 已发返利。
const reportInviterSQL = reportCohortCTE + `, reg AS (
 SELECT ua.inviter_id AS inviter_id,COUNT(*)::bigint AS new_users,COUNT(*) FILTER (WHERE cp.paid)::bigint AS paying_users,COALESCE(SUM(cr.revenue),0)::double precision AS revenue
 FROM cohort c JOIN user_affiliates ua ON ua.user_id=c.user_id AND ua.inviter_id IS NOT NULL LEFT JOIN cohort_pay cp ON cp.user_id=c.user_id LEFT JOIN cohort_rev cr ON cr.user_id=c.user_id GROUP BY ua.inviter_id
), rebate AS (
 SELECT l.user_id AS inviter_id,COALESCE(SUM(l.amount),0)::double precision AS amount FROM user_affiliate_ledger l JOIN cohort c ON c.user_id=l.source_user_id WHERE l.action='accrue' GROUP BY l.user_id
)
SELECT reg.inviter_id::text,COALESCE(u.email,''),0::bigint,reg.new_users,reg.paying_users,reg.revenue,COALESCE(rb.amount,0)
FROM reg LEFT JOIN users u ON u.id=reg.inviter_id LEFT JOIN rebate rb ON rb.inviter_id=reg.inviter_id
WHERE $3::date <= $4::date ORDER BY reg.new_users DESC,reg.revenue DESC LIMIT 20`

// 外站 host：没有渠道码的外部流量，提示值得建渠道码的来源。
const reportExternalHostSQL = reportCohortCTE + `, reg AS (
 SELECT COALESCE(NULLIF(c.referrer,''),'(direct)') AS k,COUNT(*)::bigint AS new_users,COUNT(*) FILTER (WHERE cp.paid)::bigint AS paying_users,COALESCE(SUM(cr.revenue),0)::double precision AS revenue
 FROM cohort c LEFT JOIN cohort_pay cp ON cp.user_id=c.user_id LEFT JOIN cohort_rev cr ON cr.user_id=c.user_id WHERE c.acquisition_class='other' AND c.channel_id IN (SELECT id FROM promotion_channels WHERE code='EXTERNAL') GROUP BY 1
), vis AS (
 SELECT COALESCE(NULLIF(v.referrer_host,''),'(direct)') AS k,SUM(v.visits)::bigint AS visits FROM acquisition_visits_daily v WHERE v.acquisition_class='other' AND v.channel_id IN (SELECT id FROM promotion_channels WHERE code='EXTERNAL') AND v.day >= $3::date AND v.day < $4::date GROUP BY 1
)
SELECT COALESCE(reg.k,vis.k),COALESCE(reg.k,vis.k),COALESCE(vis.visits,0),COALESCE(reg.new_users,0),COALESCE(reg.paying_users,0),COALESCE(reg.revenue,0),0::double precision
FROM reg FULL OUTER JOIN vis ON vis.k=reg.k ORDER BY COALESCE(vis.visits,0) DESC,COALESCE(reg.new_users,0) DESC LIMIT 20`

const reportPromotionSQL = `WITH attributed AS (
 SELECT a.channel_id,a.user_id,u.created_at AS user_created_at,(ua.inviter_id IS NOT NULL) AS invited
 FROM promotion_user_attributions a JOIN users u ON u.id=a.user_id LEFT JOIN user_affiliates ua ON ua.user_id=a.user_id
), eligible AS (
 SELECT * FROM attributed WHERE $3='operation' OR (user_created_at >= $1 AND user_created_at < $2)
), registrations AS (
 SELECT channel_id,COUNT(*)::bigint AS new_users,COUNT(*) FILTER (WHERE invited)::bigint AS invited_users FROM attributed WHERE user_created_at >= $1 AND user_created_at < $2 GROUP BY channel_id
), visits AS (
 SELECT channel_id,SUM(visits)::bigint AS visits FROM acquisition_visits_daily WHERE day >= $4::date AND day < $5::date GROUP BY channel_id
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
SELECT c.id,c.code,c.name,c.channel_type,c.system,COALESCE(pr.name,''),COALESCE(v.visits,0),COALESCE(reg.new_users,0),COALESCE(reg.invited_users,0),COALESCE(pay.paying_users,0),COALESCE(us.active_users,0),COALESCE(pay.recharge,0),COALESCE(us.revenue,0),COALESCE(us.upstream_cost,0),COALESCE(bs.bonus_cost,0),COALESCE(afs.affiliate_cost,0),COALESCE(cs.commission_cost,0),COALESCE(pay.payment_fee,0),COALESCE(m.marketing_cost,0)
FROM promotion_channels c LEFT JOIN promotion_promoters pr ON pr.id=c.promoter_id LEFT JOIN registrations reg ON reg.channel_id=c.id LEFT JOIN visits v ON v.channel_id=c.id LEFT JOIN payment_stats pay ON pay.channel_id=c.id LEFT JOIN usage_stats us ON us.channel_id=c.id LEFT JOIN bonus_stats bs ON bs.channel_id=c.id LEFT JOIN affiliate_stats afs ON afs.channel_id=c.id LEFT JOIN commission_stats cs ON cs.channel_id=c.id LEFT JOIN marketing m ON m.channel_id=c.id ORDER BY c.system DESC,c.id`
