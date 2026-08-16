package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// CostConfig describes a versioned operating-cost rule. It is deliberately
// separate from account/group billing multipliers: those values describe the
// upstream or customer billing path, while this model describes management
// accounting assumptions.
type CostConfig struct {
	ID               int64          `json:"id"`
	Code             string         `json:"code"`
	Name             string         `json:"name"`
	Category         string         `json:"category"`
	Amount           float64        `json:"amount"`
	Currency         string         `json:"currency"`
	ExchangeRate     float64        `json:"exchange_rate_to_billing_unit"`
	AllocationMethod string         `json:"allocation_method"`
	Frequency        string         `json:"frequency"`
	Scope            map[string]any `json:"scope"`
	EffectiveFrom    time.Time      `json:"effective_from"`
	EffectiveTo      *time.Time     `json:"effective_to,omitempty"`
	Enabled          bool           `json:"enabled"`
	Notes            string         `json:"notes"`
	CreatedBy        *int64         `json:"created_by,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type CostConfigInput struct {
	Code             string
	Name             string
	Category         string
	Amount           float64
	Currency         string
	ExchangeRate     float64
	AllocationMethod string
	Frequency        string
	Scope            map[string]any
	EffectiveFrom    time.Time
	EffectiveTo      *time.Time
	Enabled          bool
	Notes            string
}

type ExpenseEntry struct {
	ID               int64          `json:"id"`
	Category         string         `json:"category"`
	Name             string         `json:"name"`
	Amount           float64        `json:"amount"`
	Currency         string         `json:"currency"`
	ExchangeRate     float64        `json:"exchange_rate_to_billing_unit"`
	OccurredAt       time.Time      `json:"occurred_at"`
	PeriodStart      *time.Time     `json:"period_start,omitempty"`
	PeriodEnd        *time.Time     `json:"period_end,omitempty"`
	AllocationMethod string         `json:"allocation_method"`
	Scope            map[string]any `json:"scope"`
	Status           string         `json:"status"`
	Notes            string         `json:"notes"`
	CreatedBy        *int64         `json:"created_by,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type ExpenseInput struct {
	Category         string
	Name             string
	Amount           float64
	Currency         string
	ExchangeRate     float64
	OccurredAt       time.Time
	PeriodStart      *time.Time
	PeriodEnd        *time.Time
	AllocationMethod string
	Scope            map[string]any
	Notes            string
}

type ExpenseListFilter struct {
	Page      int
	PageSize  int
	Category  string
	Status    string
	StartTime *time.Time
	EndTime   *time.Time
}

type BusinessFinanceRepository interface {
	ListCostConfigs(ctx context.Context) ([]CostConfig, error)
	CreateCostConfig(ctx context.Context, input CostConfigInput, createdBy int64) (*CostConfig, error)
	UpdateCostConfig(ctx context.Context, id int64, input CostConfigInput) (*CostConfig, error)
	DisableCostConfig(ctx context.Context, id int64) error
	DeleteCostConfig(ctx context.Context, id int64) error
	ListExpenses(ctx context.Context, filter ExpenseListFilter) ([]ExpenseEntry, int, error)
	CreateExpense(ctx context.Context, input ExpenseInput, createdBy int64) (*ExpenseEntry, error)
	UpdateExpense(ctx context.Context, id int64, input ExpenseInput) (*ExpenseEntry, error)
	VoidExpense(ctx context.Context, id int64) error
	ListUpstreamCostVersions(ctx context.Context, accountID int64, model string) ([]UpstreamCostVersion, error)
	CreateUpstreamCostVersion(ctx context.Context, input UpstreamCostVersionInput, createdBy int64) (*UpstreamCostVersion, error)
	GetBusinessFinanceReport(ctx context.Context, filter FinanceReportFilter) (*FinanceReport, error)
	GetBusinessFinanceGrowth(ctx context.Context, filter FinanceReportFilter) (*FinanceGrowthReport, error)
}

type BusinessFinanceService struct {
	repo BusinessFinanceRepository
}

func NewBusinessFinanceService(repo BusinessFinanceRepository) *BusinessFinanceService {
	return &BusinessFinanceService{repo: repo}
}

func (s *BusinessFinanceService) ListCostConfigs(ctx context.Context) ([]CostConfig, error) {
	return s.repo.ListCostConfigs(ctx)
}

func (s *BusinessFinanceService) CreateCostConfig(ctx context.Context, input CostConfigInput, createdBy int64) (*CostConfig, error) {
	if err := validateCostConfigInput(&input); err != nil {
		return nil, err
	}
	return s.repo.CreateCostConfig(ctx, input, createdBy)
}

func (s *BusinessFinanceService) UpdateCostConfig(ctx context.Context, id int64, input CostConfigInput) (*CostConfig, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid cost config id")
	}
	if err := validateCostConfigInput(&input); err != nil {
		return nil, err
	}
	return s.repo.UpdateCostConfig(ctx, id, input)
}

func (s *BusinessFinanceService) DisableCostConfig(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("invalid cost config id")
	}
	return s.repo.DisableCostConfig(ctx, id)
}

func (s *BusinessFinanceService) DeleteCostConfig(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("invalid cost config id")
	}
	return s.repo.DeleteCostConfig(ctx, id)
}

func (s *BusinessFinanceService) ListExpenses(ctx context.Context, filter ExpenseListFilter) ([]ExpenseEntry, int, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	if filter.Status == "" {
		filter.Status = "active"
	}
	return s.repo.ListExpenses(ctx, filter)
}

func (s *BusinessFinanceService) CreateExpense(ctx context.Context, input ExpenseInput, createdBy int64) (*ExpenseEntry, error) {
	if err := validateExpenseInput(&input); err != nil {
		return nil, err
	}
	return s.repo.CreateExpense(ctx, input, createdBy)
}

func (s *BusinessFinanceService) UpdateExpense(ctx context.Context, id int64, input ExpenseInput) (*ExpenseEntry, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid expense id")
	}
	if err := validateExpenseInput(&input); err != nil {
		return nil, err
	}
	return s.repo.UpdateExpense(ctx, id, input)
}

func (s *BusinessFinanceService) VoidExpense(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("invalid expense id")
	}
	return s.repo.VoidExpense(ctx, id)
}

func (s *BusinessFinanceService) ListUpstreamCostVersions(ctx context.Context, accountID int64, model string) ([]UpstreamCostVersion, error) {
	model = strings.ToLower(strings.TrimSpace(model))
	if model != "" {
		if _, ok := supportedManualUpstreamCostModels[model]; !ok {
			return nil, fmt.Errorf("unsupported upstream cost model")
		}
	}
	if accountID < 0 {
		return nil, fmt.Errorf("invalid account id")
	}
	return s.repo.ListUpstreamCostVersions(ctx, accountID, model)
}

func (s *BusinessFinanceService) CreateUpstreamCostVersion(ctx context.Context, input UpstreamCostVersionInput, createdBy int64) (*UpstreamCostVersion, error) {
	if err := validateUpstreamCostVersionInput(&input); err != nil {
		return nil, err
	}
	return s.repo.CreateUpstreamCostVersion(ctx, input, createdBy)
}

func validateCostConfigInput(input *CostConfigInput) error {
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.Category = strings.TrimSpace(strings.ToLower(input.Category))
	input.Currency = normalizeCurrency(input.Currency)
	input.AllocationMethod = strings.TrimSpace(strings.ToLower(input.AllocationMethod))
	if input.Code == "" || input.Name == "" {
		return fmt.Errorf("cost config code and name are required")
	}
	if input.Amount < 0 {
		return fmt.Errorf("cost config amount cannot be negative")
	}
	if input.ExchangeRate == 0 {
		input.ExchangeRate = 1
	}
	if input.ExchangeRate < 0 {
		return fmt.Errorf("cost config exchange rate must be positive")
	}
	if !validFinanceCategory(input.Category) {
		return fmt.Errorf("unsupported cost category: %s", input.Category)
	}
	if !validAllocationMethod(input.AllocationMethod) {
		return fmt.Errorf("unsupported allocation method: %s", input.AllocationMethod)
	}
	if err := validateFrequency(input); err != nil {
		return err
	}
	if err := validateFinanceScope(input.Scope); err != nil {
		return err
	}
	if input.EffectiveFrom.IsZero() {
		input.EffectiveFrom = time.Now().UTC()
	}
	if input.EffectiveTo != nil && !input.EffectiveTo.After(input.EffectiveFrom) {
		return fmt.Errorf("effective_to must be after effective_from")
	}
	return nil
}

func validateExpenseInput(input *ExpenseInput) error {
	input.Name = strings.TrimSpace(input.Name)
	input.Category = strings.TrimSpace(strings.ToLower(input.Category))
	input.Currency = normalizeCurrency(input.Currency)
	input.AllocationMethod = strings.TrimSpace(strings.ToLower(input.AllocationMethod))
	if input.Name == "" || input.OccurredAt.IsZero() {
		return fmt.Errorf("expense name and occurred_at are required")
	}
	if input.Amount < 0 {
		return fmt.Errorf("expense amount cannot be negative")
	}
	if input.ExchangeRate == 0 {
		input.ExchangeRate = 1
	}
	if input.ExchangeRate < 0 {
		return fmt.Errorf("expense exchange rate must be positive")
	}
	if !validFinanceCategory(input.Category) {
		return fmt.Errorf("unsupported expense category: %s", input.Category)
	}
	if !validAllocationMethod(input.AllocationMethod) {
		return fmt.Errorf("unsupported allocation method: %s", input.AllocationMethod)
	}
	if err := validateFinanceScope(input.Scope); err != nil {
		return err
	}
	if input.PeriodStart != nil && input.PeriodEnd != nil && !input.PeriodEnd.After(*input.PeriodStart) {
		return fmt.Errorf("period_end must be after period_start")
	}
	return nil
}

func normalizeCurrency(currency string) string {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		return "CNY"
	}
	return currency
}

func validFinanceCategory(category string) bool {
	switch category {
	case "server", "database", "redis", "bandwidth", "domain", "compliance", "proxy", "payment_fee", "marketing", "affiliate", "account_purchase", "customer_service", "risk_reserve", "other":
		return true
	default:
		return false
	}
}

func validAllocationMethod(method string) bool {
	switch method {
	case "revenue_share", "request_share", "token_share", "account_average", "manual", "direct":
		return true
	default:
		return false
	}
}

func normalizeFrequency(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "monthly"
	}
	return value
}

func validFrequency(value string) bool {
	switch value {
	case "one_time", "daily", "monthly", "yearly":
		return true
	default:
		return false
	}
}

func validateFinanceScope(scope map[string]any) error {
	for key, value := range scope {
		switch key {
		case "group_id", "channel_id", "account_id":
			if !validScopeID(value) {
				return fmt.Errorf("scope %s must be a non-negative integer", key)
			}
		case "model":
			if strings.TrimSpace(fmt.Sprint(value)) == "" {
				return fmt.Errorf("scope model must not be empty")
			}
		default:
			return fmt.Errorf("unsupported scope key: %s", key)
		}
	}
	return nil
}

func validScopeID(value any) bool {
	switch typed := value.(type) {
	case float64:
		return typed >= 0 && typed == math.Trunc(typed)
	case json.Number:
		parsed, err := strconv.ParseInt(typed.String(), 10, 64)
		return err == nil && parsed >= 0
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return err == nil && parsed >= 0
	case int:
		return typed >= 0
	case int64:
		return typed >= 0
	case uint:
		return true
	case uint64:
		return typed <= uint64(1<<63-1)
	default:
		return false
	}
}

// FinanceReportFilter is shared by the management dashboard and reports.
// Dates are UTC half-open intervals [StartTime, EndTime).
type FinanceReportFilter struct {
	StartTime time.Time
	EndTime   time.Time
	Dimension string
	GroupID   int64
	ChannelID int64
	Model     string
	MinMargin float64
}

type FinanceMetric struct {
	Requests        int64   `json:"requests"`
	Tokens          int64   `json:"tokens"`
	Revenue         float64 `json:"revenue"`
	DirectCost      float64 `json:"direct_cost"`
	OperatingCost   float64 `json:"operating_cost"`
	TotalCost       float64 `json:"total_cost"`
	GrossProfit     float64 `json:"gross_profit"`
	OperatingProfit float64 `json:"operating_profit"`
	GrossMargin     float64 `json:"gross_margin"`
	OperatingMargin float64 `json:"operating_margin"`
	CostMultiplier  float64 `json:"cost_multiplier"`
}

type FinanceReportRow struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	FinanceMetric
}

type FinanceReport struct {
	StartTime  time.Time              `json:"start_time"`
	EndTime    time.Time              `json:"end_time"`
	Dimension  string                 `json:"dimension"`
	Summary    FinanceMetric          `json:"summary"`
	Rows       []FinanceReportRow     `json:"rows"`
	Trend      []FinanceTrendPoint    `json:"trend"`
	Components []FinanceCostComponent `json:"components"`
	Alerts     []FinanceRiskAlert     `json:"alerts"`
}

type FinanceTrendPoint struct {
	Date string `json:"date"`
	FinanceMetric
}

type FinanceCostComponent struct {
	Category         string  `json:"category"`
	Name             string  `json:"name"`
	Amount           float64 `json:"amount"`
	OriginalAmount   float64 `json:"original_amount,omitempty"`
	Currency         string  `json:"currency,omitempty"`
	ExchangeRate     float64 `json:"exchange_rate_to_billing_unit,omitempty"`
	AllocationMethod string  `json:"allocation_method"`
	Source           string  `json:"source"`
}

type FinanceGrowthSource struct {
	Source      string  `json:"source"`
	NewUsers    int64   `json:"new_users"`
	ActiveUsers int64   `json:"active_users"`
	PayingUsers int64   `json:"paying_users"`
	Revenue     float64 `json:"revenue"`
	Recharge    float64 `json:"recharge"`
}

type FinanceGrowthReport struct {
	StartTime      time.Time             `json:"start_time"`
	EndTime        time.Time             `json:"end_time"`
	NewUsers       int64                 `json:"new_users"`
	ActiveUsers    int64                 `json:"active_users"`
	OnlineUsers    int64                 `json:"online_users"`
	PayingUsers    int64                 `json:"paying_users"`
	RechargeAmount float64               `json:"recharge_amount"`
	Revenue        float64               `json:"revenue"`
	MarketingCost  float64               `json:"marketing_cost"`
	AffiliateCost  float64               `json:"affiliate_cost"`
	CAC            float64               `json:"cac"`
	LTV            float64               `json:"ltv"`
	ROI            float64               `json:"roi"`
	BySource       []FinanceGrowthSource `json:"by_source"`
}

type FinanceRiskAlert struct {
	Severity        string  `json:"severity"`
	Dimension       string  `json:"dimension"`
	Key             string  `json:"key"`
	Name            string  `json:"name"`
	Reason          string  `json:"reason"`
	OperatingProfit float64 `json:"operating_profit"`
	OperatingMargin float64 `json:"operating_margin"`
	CostMultiplier  float64 `json:"cost_multiplier"`
}

func (s *BusinessFinanceService) GetBusinessFinanceReport(ctx context.Context, filter FinanceReportFilter) (*FinanceReport, error) {
	filter = normalizeFinanceReportFilter(filter)
	return s.repo.GetBusinessFinanceReport(ctx, filter)
}

func (s *BusinessFinanceService) GetBusinessFinanceGrowth(ctx context.Context, filter FinanceReportFilter) (*FinanceGrowthReport, error) {
	filter = normalizeFinanceReportFilter(filter)
	return s.repo.GetBusinessFinanceGrowth(ctx, filter)
}

func normalizeFinanceReportFilter(filter FinanceReportFilter) FinanceReportFilter {
	if filter.EndTime.IsZero() {
		filter.EndTime = time.Now().UTC()
	}
	if filter.StartTime.IsZero() {
		filter.StartTime = filter.EndTime.AddDate(0, 0, -30)
	}
	if !filter.EndTime.After(filter.StartTime) {
		filter.EndTime = filter.StartTime.Add(24 * time.Hour)
	}
	switch filter.Dimension {
	case "group", "channel", "model", "account":
	default:
		filter.Dimension = "group"
	}
	return filter
}

func financeMetricFromValues(requests, tokens int64, revenue, directCost, operatingCost float64) FinanceMetric {
	totalCost := directCost + operatingCost
	metric := FinanceMetric{
		Requests: requests, Tokens: tokens, Revenue: revenue,
		DirectCost: directCost, OperatingCost: operatingCost, TotalCost: totalCost,
		GrossProfit: revenue - directCost, OperatingProfit: revenue - totalCost,
	}
	if revenue > 0 {
		metric.GrossMargin = metric.GrossProfit / revenue
		metric.OperatingMargin = metric.OperatingProfit / revenue
	}
	if directCost > 0 {
		metric.CostMultiplier = totalCost / directCost
	}
	return metric
}

func validateFrequency(input *CostConfigInput) error {
	input.Frequency = normalizeFrequency(input.Frequency)
	if !validFrequency(input.Frequency) {
		return fmt.Errorf("unsupported cost frequency: %s", input.Frequency)
	}
	return nil
}
