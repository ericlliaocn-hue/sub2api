package service

import (
	"context"
	"math"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type promotionSourceContextKey struct{}

func WithPromotionSource(ctx context.Context, source string) context.Context {
	return context.WithValue(ctx, promotionSourceContextKey{}, strings.TrimSpace(source))
}

func promotionSourceFromContext(ctx context.Context) string {
	if value, ok := ctx.Value(promotionSourceContextKey{}).(string); ok {
		return value
	}
	return ""
}

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
	ChannelID      int64   `json:"channel_id"`
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	ChannelType    string  `json:"channel_type"`
	PromoterName   string  `json:"promoter_name"`
	NewUsers       int64   `json:"new_users"`
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
type PromotionReport struct {
	StartTime time.Time            `json:"start_time"`
	EndTime   time.Time            `json:"end_time"`
	Mode      string               `json:"mode"`
	Rows      []PromotionReportRow `json:"rows"`
}

const (
	PromotionReportModeOperation   = "operation"
	PromotionReportModeAcquisition = "acquisition"
)

type PromotionAttributionEvent struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	UserEmail     string    `json:"user_email"`
	RequestedCode string    `json:"requested_code"`
	ChannelID     *int64    `json:"channel_id,omitempty"`
	ChannelName   string    `json:"channel_name"`
	Outcome       string    `json:"outcome"`
	Detail        string    `json:"detail"`
	CreatedAt     time.Time `json:"created_at"`
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
	AttributeUser(context.Context, int64, string) error
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
func (s *PromotionService) AttributeUser(ctx context.Context, userID int64, code string) error {
	code = normalizePromotionCode(code)
	if code == "" {
		return nil
	}
	return s.repo.AttributeUser(ctx, userID, code)
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
	in.ChannelType = strings.TrimSpace(in.ChannelType)
	in.Notes = strings.TrimSpace(in.Notes)
	if in.ChannelType == "" {
		in.ChannelType = "other"
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
