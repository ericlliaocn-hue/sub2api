package service

import (
	"context"
	"strings"
	"time"
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
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Contact        string    `json:"contact"`
	CommissionRate float64   `json:"commission_rate"`
	Enabled        bool      `json:"enabled"`
	Notes          string    `json:"notes"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
type PromotionChannel struct {
	ID           int64     `json:"id"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	ChannelType  string    `json:"channel_type"`
	PromoterID   *int64    `json:"promoter_id,omitempty"`
	PromoterName string    `json:"promoter_name,omitempty"`
	Enabled      bool      `json:"enabled"`
	Notes        string    `json:"notes"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type PromotionPromoterInput struct {
	Name           string
	Contact        string
	CommissionRate float64
	Enabled        bool
	Notes          string
}
type PromotionChannelInput struct {
	Code        string
	Name        string
	ChannelType string
	PromoterID  *int64
	Enabled     bool
	Notes       string
}
type PromotionReportRow struct {
	ChannelID     int64   `json:"channel_id"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	ChannelType   string  `json:"channel_type"`
	PromoterName  string  `json:"promoter_name"`
	NewUsers      int64   `json:"new_users"`
	PayingUsers   int64   `json:"paying_users"`
	ActiveUsers   int64   `json:"active_users"`
	Recharge      float64 `json:"recharge"`
	Revenue       float64 `json:"revenue"`
	MarketingCost float64 `json:"marketing_cost"`
	Profit        float64 `json:"profit"`
	CAC           float64 `json:"cac"`
	ROI           float64 `json:"roi"`
}
type PromotionReport struct {
	StartTime time.Time            `json:"start_time"`
	EndTime   time.Time            `json:"end_time"`
	Rows      []PromotionReportRow `json:"rows"`
}
type PromotionRepository interface {
	ListPromoters(context.Context) ([]PromotionPromoter, error)
	CreatePromoter(context.Context, PromotionPromoterInput) (*PromotionPromoter, error)
	UpdatePromoter(context.Context, int64, PromotionPromoterInput) (*PromotionPromoter, error)
	ListChannels(context.Context) ([]PromotionChannel, error)
	CreateChannel(context.Context, PromotionChannelInput) (*PromotionChannel, error)
	UpdateChannel(context.Context, int64, PromotionChannelInput) (*PromotionChannel, error)
	AttributeUser(context.Context, int64, string) error
	Report(context.Context, time.Time, time.Time) (*PromotionReport, error)
}
type PromotionService struct{ repo PromotionRepository }

func NewPromotionService(repo PromotionRepository) *PromotionService {
	return &PromotionService{repo: repo}
}
func (s *PromotionService) ListPromoters(ctx context.Context) ([]PromotionPromoter, error) {
	return s.repo.ListPromoters(ctx)
}
func (s *PromotionService) CreatePromoter(ctx context.Context, in PromotionPromoterInput) (*PromotionPromoter, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, ErrInvalidInput
	}
	return s.repo.CreatePromoter(ctx, in)
}
func (s *PromotionService) UpdatePromoter(ctx context.Context, id int64, in PromotionPromoterInput) (*PromotionPromoter, error) {
	in.Name = strings.TrimSpace(in.Name)
	return s.repo.UpdatePromoter(ctx, id, in)
}
func (s *PromotionService) ListChannels(ctx context.Context) ([]PromotionChannel, error) {
	return s.repo.ListChannels(ctx)
}
func (s *PromotionService) CreateChannel(ctx context.Context, in PromotionChannelInput) (*PromotionChannel, error) {
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	if in.Code == "" || in.Name == "" {
		return nil, ErrInvalidInput
	}
	return s.repo.CreateChannel(ctx, in)
}
func (s *PromotionService) UpdateChannel(ctx context.Context, id int64, in PromotionChannelInput) (*PromotionChannel, error) {
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	return s.repo.UpdateChannel(ctx, id, in)
}
func (s *PromotionService) AttributeUser(ctx context.Context, userID int64, code string) error {
	if strings.TrimSpace(code) == "" {
		return nil
	}
	return s.repo.AttributeUser(ctx, userID, code)
}
func (s *PromotionService) Report(ctx context.Context, start, end time.Time) (*PromotionReport, error) {
	return s.repo.Report(ctx, start, end)
}
