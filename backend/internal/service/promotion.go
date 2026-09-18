package service

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

type PromotionPromoter struct {
	ID                   int64     `json:"id"`
	Name                 string    `json:"name"`
	Contact              string    `json:"contact"`
	CommissionRate       float64   `json:"commission_rate"`
	CommissionFreezeDays int       `json:"commission_freeze_days"`
	Enabled              bool      `json:"enabled"`
	Notes                string    `json:"notes"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}
type PromotionChannel struct {
	ID             int64     `json:"id"`
	Code           string    `json:"code"`
	Name           string    `json:"name"`
	ChannelType    string    `json:"channel_type"`
	PromoterID     *int64    `json:"promoter_id,omitempty"`
	PromoterName   string    `json:"promoter_name,omitempty"`
	CommissionRate *float64  `json:"commission_rate,omitempty"`
	Enabled        bool      `json:"enabled"`
	System         bool      `json:"system"`
	Notes          string    `json:"notes"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
type PromotionPromoterInput struct {
	Name                 string
	Contact              string
	CommissionRate       float64
	CommissionFreezeDays int
	Enabled              bool
	Notes                string
}
type PromotionChannelInput struct {
	Code           string
	Name           string
	ChannelType    string
	PromoterID     *int64
	CommissionRate *float64
	Enabled        bool
	Notes          string
}
type PromotionReportRow struct {
	ChannelID        int64   `json:"channel_id"`
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	ChannelType      string  `json:"channel_type"`
	AcquisitionClass string  `json:"acquisition_class"`
	System           bool    `json:"system"`
	PromoterName     string  `json:"promoter_name"`
	Visits           int64   `json:"visits"`
	ConversionRate   float64 `json:"conversion_rate"`
	NewUsers         int64   `json:"new_users"`
	InvitedUsers     int64   `json:"invited_users"`
	PayingUsers      int64   `json:"paying_users"`
	ActiveUsers      int64   `json:"active_users"`
	Recharge         float64 `json:"recharge"`
	Revenue          float64 `json:"revenue"`
	UpstreamCost     float64 `json:"upstream_cost"`
	BonusCost        float64 `json:"bonus_cost"`
	AffiliateCost    float64 `json:"affiliate_cost"`
	CommissionCost   float64 `json:"commission_cost"`
	PaymentFee       float64 `json:"payment_fee"`
	MarketingCost    float64 `json:"marketing_cost"`
	Profit           float64 `json:"profit"`
	CAC              float64 `json:"cac"`
	LTV              float64 `json:"ltv"`
	ROI              float64 `json:"roi"`
}

// PromotionReportClassRow 四个获客分类之一的汇总（官网 / SEO / 邀请 / 其他）。
type PromotionReportClassRow struct {
	Class          string  `json:"class"`
	Visits         int64   `json:"visits"`
	ConversionRate float64 `json:"conversion_rate"`
	NewUsers       int64   `json:"new_users"`
	NewUsersShare  float64 `json:"new_users_share"`
	InvitedUsers   int64   `json:"invited_users"`
	PayingUsers    int64   `json:"paying_users"`
	ActiveUsers    int64   `json:"active_users"`
	Recharge       float64 `json:"recharge"`
	Revenue        float64 `json:"revenue"`
	UpstreamCost   float64 `json:"upstream_cost"`
	BonusCost      float64 `json:"bonus_cost"`
	AffiliateCost  float64 `json:"affiliate_cost"`
	CommissionCost float64 `json:"commission_cost"`
	PaymentFee     float64 `json:"payment_fee"`
	MarketingCost  float64 `json:"marketing_cost"`
	Profit         float64 `json:"profit"`
	CAC            float64 `json:"cac"`
	LTV            float64 `json:"ltv"`
	ROI            float64 `json:"roi"`
}

// PromotionReportTotals 区间总览。UnattributedUsers > 0 说明归因漏人，前端要报警。
type PromotionReportTotals struct {
	Visits            int64   `json:"visits"`
	ConversionRate    float64 `json:"conversion_rate"`
	RegisteredUsers   int64   `json:"registered_users"`
	NewUsers          int64   `json:"new_users"`
	UnattributedUsers int64   `json:"unattributed_users"`
	InvitedUsers      int64   `json:"invited_users"`
	PayingUsers       int64   `json:"paying_users"`
	ActiveUsers       int64   `json:"active_users"`
	Recharge          float64 `json:"recharge"`
	Revenue           float64 `json:"revenue"`
	Profit            float64 `json:"profit"`
}

// PromotionBreakdownRow 下钻明细：SEO 引擎 / 落地页、邀请人、外站 host。
type PromotionBreakdownRow struct {
	Key         string  `json:"key"`
	Label       string  `json:"label"`
	Visits      int64   `json:"visits"`
	NewUsers    int64   `json:"new_users"`
	PayingUsers int64   `json:"paying_users"`
	Revenue     float64 `json:"revenue"`
	Extra       float64 `json:"extra"`
}

type PromotionReport struct {
	StartTime       time.Time                 `json:"start_time"`
	EndTime         time.Time                 `json:"end_time"`
	Mode            string                    `json:"mode"`
	Totals          PromotionReportTotals     `json:"totals"`
	Classes         []PromotionReportClassRow `json:"classes"`
	Rows            []PromotionReportRow      `json:"rows"`
	SEOEngines      []PromotionBreakdownRow   `json:"seo_engines"`
	SEOLandingPages []PromotionBreakdownRow   `json:"seo_landing_pages"`
	Inviters        []PromotionBreakdownRow   `json:"inviters"`
	ExternalHosts   []PromotionBreakdownRow   `json:"external_hosts"`
}

// AcquisitionClassOrder 报表固定顺序。
var AcquisitionClassOrder = []string{AcquisitionClassOfficial, AcquisitionClassSEO, AcquisitionClassInvite, AcquisitionClassOther}

const (
	PromotionReportModeOperation   = "operation"
	PromotionReportModeAcquisition = "acquisition"
)

type PromotionAttributionEvent struct {
	ID               int64     `json:"id"`
	UserID           int64     `json:"user_id"`
	UserEmail        string    `json:"user_email"`
	RequestedCode    string    `json:"requested_code"`
	ChannelID        *int64    `json:"channel_id,omitempty"`
	ChannelName      string    `json:"channel_name"`
	AcquisitionClass string    `json:"acquisition_class"`
	Outcome          string    `json:"outcome"`
	Detail           string    `json:"detail"`
	CreatedAt        time.Time `json:"created_at"`
}

// PromotionAttributionInput 是写入归因行的结构化输入（服务端裁定后的结论）。
type PromotionAttributionInput struct {
	UserID       int64
	ChannelCode  string
	Class        string
	LandingPath  string
	ReferrerHost string
	UTM          map[string]string
	Evidence     map[string]any
	FirstTouchAt time.Time
}

// PromotionAttributionEventInput 审计事件输入。
type PromotionAttributionEventInput struct {
	UserID        int64
	RequestedCode string
	ChannelID     *int64
	Class         string
	Outcome       string
	Detail        string
}

// AcquisitionVisitInput 一次落地访问的聚合键。
type AcquisitionVisitInput struct {
	Day          time.Time
	Class        string
	ChannelCode  string
	Engine       string
	ReferrerHost string
	LandingPath  string
}

type PromotionCommission struct {
	ID             int64      `json:"id"`
	PaymentOrderID int64      `json:"payment_order_id"`
	UserID         int64      `json:"user_id"`
	UserEmail      string     `json:"user_email"`
	ChannelID      int64      `json:"channel_id"`
	ChannelCode    string     `json:"channel_code"`
	ChannelName    string     `json:"channel_name"`
	PromoterID     int64      `json:"promoter_id"`
	PromoterName   string     `json:"promoter_name"`
	BaseAmount     float64    `json:"base_amount"`
	CommissionRate float64    `json:"commission_rate"`
	Amount         float64    `json:"amount"`
	ReversedAmount float64    `json:"reversed_amount"`
	Currency       string     `json:"currency"`
	Status         string     `json:"status"`
	FrozenUntil    *time.Time `json:"frozen_until,omitempty"`
	SettlementID   *int64     `json:"settlement_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type PromotionSettlement struct {
	ID           int64      `json:"id"`
	PromoterID   int64      `json:"promoter_id"`
	PromoterName string     `json:"promoter_name"`
	PeriodEnd    time.Time  `json:"period_end"`
	Amount       float64    `json:"amount"`
	Status       string     `json:"status"`
	Notes        string     `json:"notes"`
	PaidAt       *time.Time `json:"paid_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type PromotionSettlementInput struct {
	PromoterID int64
	PeriodEnd  time.Time
	Notes      string
}
type PromotionRepository interface {
	ListPromoters(context.Context) ([]PromotionPromoter, error)
	CreatePromoter(context.Context, PromotionPromoterInput) (*PromotionPromoter, error)
	UpdatePromoter(context.Context, int64, PromotionPromoterInput) (*PromotionPromoter, error)
	ListChannels(context.Context) ([]PromotionChannel, error)
	CreateChannel(context.Context, PromotionChannelInput) (*PromotionChannel, error)
	UpdateChannel(context.Context, int64, PromotionChannelInput) (*PromotionChannel, error)
	GetChannelByCode(context.Context, string) (*PromotionChannel, error)
	// AttributeUser 写入归因；已有归因时返回 false 且不覆盖（注册后不可变）。
	AttributeUser(context.Context, PromotionAttributionInput) (bool, error)
	RecordAttributionEvent(context.Context, PromotionAttributionEventInput) error
	RecordVisit(context.Context, AcquisitionVisitInput) error
	ListAttributionEvents(context.Context, int) ([]PromotionAttributionEvent, error)
	ListCommissions(context.Context, int64, string, int) ([]PromotionCommission, error)
	ListSettlements(context.Context, int64, int) ([]PromotionSettlement, error)
	CreateSettlement(context.Context, PromotionSettlementInput) (*PromotionSettlement, error)
	UpdateSettlementStatus(context.Context, int64, string) (*PromotionSettlement, error)
	Report(context.Context, time.Time, time.Time, string) (*PromotionReport, error)
}
type PromotionService struct{ repo PromotionRepository }

func NewPromotionService(repo PromotionRepository) *PromotionService {
	return &PromotionService{repo: repo}
}
func (s *PromotionService) ListPromoters(ctx context.Context) ([]PromotionPromoter, error) {
	return s.repo.ListPromoters(ctx)
}
func (s *PromotionService) CreatePromoter(ctx context.Context, in PromotionPromoterInput) (*PromotionPromoter, error) {
	if err := normalizePromotionPromoterInput(&in); err != nil {
		return nil, err
	}
	return s.repo.CreatePromoter(ctx, in)
}
func (s *PromotionService) UpdatePromoter(ctx context.Context, id int64, in PromotionPromoterInput) (*PromotionPromoter, error) {
	if id <= 0 {
		return nil, ErrInvalidInput
	}
	if err := normalizePromotionPromoterInput(&in); err != nil {
		return nil, err
	}
	return s.repo.UpdatePromoter(ctx, id, in)
}
func (s *PromotionService) ListChannels(ctx context.Context) ([]PromotionChannel, error) {
	return s.repo.ListChannels(ctx)
}
func (s *PromotionService) CreateChannel(ctx context.Context, in PromotionChannelInput) (*PromotionChannel, error) {
	if err := normalizePromotionChannelInput(&in); err != nil {
		return nil, err
	}
	return s.repo.CreateChannel(ctx, in)
}
func (s *PromotionService) UpdateChannel(ctx context.Context, id int64, in PromotionChannelInput) (*PromotionChannel, error) {
	if id <= 0 {
		return nil, ErrInvalidInput
	}
	if err := normalizePromotionChannelInput(&in); err != nil {
		return nil, err
	}
	return s.repo.UpdateChannel(ctx, id, in)
}
// AttributeUser 兼容旧调用：只带渠道编码，走完整裁定（空编码也会归因到官网）。
func (s *PromotionService) AttributeUser(ctx context.Context, userID int64, code string) error {
	touch := AcquisitionTouchFromContext(ctx)
	if code = normalizePromotionCode(code); code != "" {
		touch.Source = code
	}
	return s.AttributeAcquisition(ctx, userID, touch, 0)
}

// AttributeAcquisition 是注册成功后的唯一归因入口：每个新用户都要有一行，默认官网。
// 失败只记日志，绝不影响注册。
func (s *PromotionService) AttributeAcquisition(ctx context.Context, userID int64, touch AcquisitionTouch, actorUserID int64) error {
	return s.attribute(ctx, userID, touch, AcquisitionResolveOptions{ActorUserID: actorUserID, AdminCreated: actorUserID > 0})
}

// AttributeManualCreation 管理员 / API 建号：不看任何落地证据，直接归 MANUAL（其他）。
func (s *PromotionService) AttributeManualCreation(ctx context.Context, userID int64, actorUserID int64) error {
	return s.attribute(ctx, userID, AcquisitionTouch{}, AcquisitionResolveOptions{ActorUserID: actorUserID, AdminCreated: true})
}

func (s *PromotionService) attribute(ctx context.Context, userID int64, touch AcquisitionTouch, opts AcquisitionResolveOptions) error {
	if s == nil || s.repo == nil || userID <= 0 {
		return nil
	}
	actorUserID := opts.ActorUserID
	if !opts.AdminCreated {
		touch = touch.Merge(AcquisitionTouchFromContext(ctx))
	}
	touch.Source = normalizePromotionCode(touch.Source)
	if host := acquisitionSiteHostFromContext(ctx); host != "" {
		opts.SiteHosts = append(opts.SiteHosts, host)
	}
	if touch.Source != "" && !opts.AdminCreated {
		channel, err := s.repo.GetChannelByCode(ctx, touch.Source)
		switch {
		case err == nil && channel != nil && channel.Enabled:
			opts.SourceChannel = channel
		case err == nil && channel != nil:
			_ = s.repo.RecordAttributionEvent(ctx, PromotionAttributionEventInput{UserID: userID, RequestedCode: touch.Source, ChannelID: &channel.ID, Outcome: "channel_disabled", Detail: "channel is disabled; fell through to default rules"})
		case errors.Is(err, sql.ErrNoRows):
			_ = s.repo.RecordAttributionEvent(ctx, PromotionAttributionEventInput{UserID: userID, RequestedCode: touch.Source, Outcome: "invalid_code", Detail: "channel code does not exist; fell through to default rules"})
		case err != nil:
			logger.LegacyPrintf("service.promotion", "[Promotion] lookup channel %q failed: %v", touch.Source, err)
		}
	}
	result := ResolveAcquisition(touch, opts)
	created, err := s.repo.AttributeUser(ctx, PromotionAttributionInput{
		UserID:       userID,
		ChannelCode:  result.ChannelCode,
		Class:        result.Class,
		LandingPath:  touch.LandingPath,
		ReferrerHost: touch.ReferrerHost,
		UTM:          acquisitionUTMMap(touch),
		Evidence:     acquisitionEvidence(result, actorUserID),
		FirstTouchAt: touch.FirstTouchAt,
	})
	if err != nil {
		logger.LegacyPrintf("service.promotion", "[Promotion] attribute user %d failed: %v", userID, err)
		return err
	}
	outcome, detail := "resolved", result.Reason
	if result.Reason == "default_official" || result.Reason == "invalid_source_default_official" {
		outcome = "default_official"
	}
	if !created {
		outcome, detail = "already_attributed", "existing attribution retained"
	}
	var channelID *int64
	if opts.SourceChannel != nil && result.ChannelCode == opts.SourceChannel.Code {
		channelID = &opts.SourceChannel.ID
	}
	_ = s.repo.RecordAttributionEvent(ctx, PromotionAttributionEventInput{UserID: userID, RequestedCode: touch.Source, ChannelID: channelID, Class: result.Class, Outcome: outcome, Detail: detail})
	return nil
}

// RecordLandingVisit 信标端点：裁定分类并按天累加访问。
func (s *PromotionService) RecordLandingVisit(ctx context.Context, touch AcquisitionTouch, now time.Time) (AcquisitionResult, error) {
	touch.Source = normalizePromotionCode(touch.Source)
	opts := AcquisitionResolveOptions{}
	if host := acquisitionSiteHostFromContext(ctx); host != "" {
		opts.SiteHosts = append(opts.SiteHosts, host)
	}
	if touch.Source != "" {
		if channel, err := s.repo.GetChannelByCode(ctx, touch.Source); err == nil && channel != nil && channel.Enabled {
			opts.SourceChannel = channel
		}
	}
	result := ResolveAcquisition(touch, opts)
	err := s.repo.RecordVisit(ctx, AcquisitionVisitInput{
		Day:          now,
		Class:        result.Class,
		ChannelCode:  result.ChannelCode,
		Engine:       result.Engine,
		ReferrerHost: result.ReferrerHost,
		LandingPath:  touch.LandingPath,
	})
	return result, err
}

func acquisitionUTMMap(touch AcquisitionTouch) map[string]string {
	out := map[string]string{}
	if touch.UTMSource != "" {
		out["source"] = touch.UTMSource
	}
	if touch.UTMMedium != "" {
		out["medium"] = touch.UTMMedium
	}
	if touch.UTMCampaign != "" {
		out["campaign"] = touch.UTMCampaign
	}
	return out
}

func acquisitionEvidence(result AcquisitionResult, actorUserID int64) map[string]any {
	evidence := map[string]any{"reason": result.Reason, "level": result.Level}
	if result.Engine != "" {
		evidence["engine"] = result.Engine
	}
	if result.ReferrerHost != "" {
		evidence["referrer_host"] = result.ReferrerHost
	}
	if result.Paid {
		evidence["paid"] = true
	}
	if result.Touch.Source != "" {
		evidence["requested_source"] = result.Touch.Source
	}
	if result.Touch.AffCode != "" {
		evidence["aff"] = result.Touch.AffCode
	}
	if !result.Touch.FirstTouchAt.IsZero() {
		evidence["first_touch_at"] = result.Touch.FirstTouchAt.UTC().Format(time.RFC3339)
	}
	if actorUserID > 0 {
		evidence["actor_user_id"] = actorUserID
	}
	return evidence
}
func (s *PromotionService) ListAttributionEvents(ctx context.Context, limit int) ([]PromotionAttributionEvent, error) {
	return s.repo.ListAttributionEvents(ctx, clampPromotionLimit(limit))
}
func (s *PromotionService) ListCommissions(ctx context.Context, promoterID int64, status string, limit int) ([]PromotionCommission, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "" && status != "frozen" && status != "available" && status != "settled" && status != "reversed" {
		return nil, ErrInvalidInput
	}
	return s.repo.ListCommissions(ctx, promoterID, status, clampPromotionLimit(limit))
}
func (s *PromotionService) ListSettlements(ctx context.Context, promoterID int64, limit int) ([]PromotionSettlement, error) {
	return s.repo.ListSettlements(ctx, promoterID, clampPromotionLimit(limit))
}
func (s *PromotionService) CreateSettlement(ctx context.Context, in PromotionSettlementInput) (*PromotionSettlement, error) {
	if in.PromoterID <= 0 || in.PeriodEnd.IsZero() || in.PeriodEnd.After(time.Now().Add(time.Minute)) {
		return nil, ErrInvalidInput
	}
	in.Notes = strings.TrimSpace(in.Notes)
	return s.repo.CreateSettlement(ctx, in)
}
func (s *PromotionService) UpdateSettlementStatus(ctx context.Context, id int64, status string) (*PromotionSettlement, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if id <= 0 || (status != "paid" && status != "cancelled") {
		return nil, ErrInvalidInput
	}
	return s.repo.UpdateSettlementStatus(ctx, id, status)
}
func (s *PromotionService) Report(ctx context.Context, start, end time.Time, mode string) (*PromotionReport, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = PromotionReportModeOperation
	}
	if mode != PromotionReportModeOperation && mode != PromotionReportModeAcquisition {
		return nil, ErrInvalidInput
	}
	if start.IsZero() || end.IsZero() || !start.Before(end) {
		return nil, ErrInvalidInput
	}
	return s.repo.Report(ctx, start, end, mode)
}

func normalizePromotionPromoterInput(in *PromotionPromoterInput) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Contact = strings.TrimSpace(in.Contact)
	in.Notes = strings.TrimSpace(in.Notes)
	if in.Name == "" || !validPromotionRate(in.CommissionRate) || in.CommissionFreezeDays < 0 || in.CommissionFreezeDays > 365 {
		return ErrInvalidInput
	}
	return nil
}

func normalizePromotionChannelInput(in *PromotionChannelInput) error {
	in.Code = normalizePromotionCode(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	in.ChannelType = strings.ToLower(strings.TrimSpace(in.ChannelType))
	in.Notes = strings.TrimSpace(in.Notes)
	if in.ChannelType == "" {
		in.ChannelType = AcquisitionClassOther
	}
	if !isValidAcquisitionClass(in.ChannelType) {
		return infraerrors.BadRequest("PROMOTION_CHANNEL_TYPE_INVALID", "channel type must be one of official, seo, invite, other")
	}
	if in.Code == "" || in.Name == "" || len(in.Code) > 64 || len(in.Name) > 128 {
		return ErrInvalidInput
	}
	for i := range in.Code {
		c := in.Code[i]
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return infraerrors.BadRequest("PROMOTION_CODE_INVALID", "channel code may only contain letters, numbers, underscore and dash")
		}
	}
	if in.CommissionRate != nil && !validPromotionRate(*in.CommissionRate) {
		return ErrInvalidInput
	}
	return nil
}

func validPromotionRate(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 100
}

func normalizePromotionCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func clampPromotionLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}

var ErrPromotionSettlementEmpty = infraerrors.BadRequest("PROMOTION_SETTLEMENT_EMPTY", "no available commission entries for this settlement")
var ErrPromotionSettlementState = infraerrors.Conflict("PROMOTION_SETTLEMENT_STATE", "settlement status cannot be changed")
