package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type BusinessFinanceHandler struct {
	financeService *service.BusinessFinanceService
}

func NewBusinessFinanceHandler(financeService *service.BusinessFinanceService) *BusinessFinanceHandler {
	return &BusinessFinanceHandler{financeService: financeService}
}

type businessCostConfigRequest struct {
	Code             string         `json:"code"`
	Name             string         `json:"name"`
	Category         string         `json:"category"`
	Amount           float64        `json:"amount"`
	Currency         string         `json:"currency"`
	ExchangeRate     float64        `json:"exchange_rate_to_billing_unit"`
	AllocationMethod string         `json:"allocation_method"`
	Frequency        string         `json:"frequency"`
	Scope            map[string]any `json:"scope"`
	EffectiveFrom    *time.Time     `json:"effective_from"`
	EffectiveTo      *time.Time     `json:"effective_to"`
	Enabled          *bool          `json:"enabled"`
	Notes            string         `json:"notes"`
}

type businessExpenseRequest struct {
	Category         string         `json:"category"`
	Name             string         `json:"name"`
	Amount           float64        `json:"amount"`
	Currency         string         `json:"currency"`
	ExchangeRate     float64        `json:"exchange_rate_to_billing_unit"`
	OccurredAt       *time.Time     `json:"occurred_at"`
	PeriodStart      *time.Time     `json:"period_start"`
	PeriodEnd        *time.Time     `json:"period_end"`
	AllocationMethod string         `json:"allocation_method"`
	Scope            map[string]any `json:"scope"`
	Notes            string         `json:"notes"`
}

func (h *BusinessFinanceHandler) ListCostConfigs(c *gin.Context) {
	items, err := h.financeService.ListCostConfigs(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *BusinessFinanceHandler) CreateCostConfig(c *gin.Context) {
	var req businessCostConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	input, err := toCostConfigInput(req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.financeService.CreateCostConfig(c.Request.Context(), input, financeOperatorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *BusinessFinanceHandler) UpdateCostConfig(c *gin.Context) {
	id, ok := parseFinanceID(c, "id")
	if !ok {
		return
	}
	var req businessCostConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	input, err := toCostConfigInput(req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.financeService.UpdateCostConfig(c.Request.Context(), id, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *BusinessFinanceHandler) DisableCostConfig(c *gin.Context) {
	id, ok := parseFinanceID(c, "id")
	if !ok {
		return
	}
	if err := h.financeService.DisableCostConfig(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"id": id, "enabled": false})
}

func (h *BusinessFinanceHandler) ListExpenses(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := service.ExpenseListFilter{
		Page:     page,
		PageSize: pageSize,
		Category: strings.TrimSpace(strings.ToLower(c.Query("category"))),
		Status:   strings.TrimSpace(strings.ToLower(c.DefaultQuery("status", "active"))),
	}
	var err error
	filter.StartTime, err = parseFinanceQueryTime(c.Query("start_time"))
	if err != nil {
		response.BadRequest(c, "Invalid start_time")
		return
	}
	filter.EndTime, err = parseFinanceQueryTime(c.Query("end_time"))
	if err != nil {
		response.BadRequest(c, "Invalid end_time")
		return
	}
	items, total, err := h.financeService.ListExpenses(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, int64(total), page, pageSize)
}

func (h *BusinessFinanceHandler) CreateExpense(c *gin.Context) {
	var req businessExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	input, err := toExpenseInput(req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.financeService.CreateExpense(c.Request.Context(), input, financeOperatorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *BusinessFinanceHandler) UpdateExpense(c *gin.Context) {
	id, ok := parseFinanceID(c, "id")
	if !ok {
		return
	}
	var req businessExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	input, err := toExpenseInput(req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.financeService.UpdateExpense(c.Request.Context(), id, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *BusinessFinanceHandler) VoidExpense(c *gin.Context) {
	id, ok := parseFinanceID(c, "id")
	if !ok {
		return
	}
	if err := h.financeService.VoidExpense(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"id": id, "status": "void"})
}

func (h *BusinessFinanceHandler) GetReport(c *gin.Context) {
	filter, err := parseBusinessFinanceReportFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	report, err := h.financeService.GetBusinessFinanceReport(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, report)
}

func (h *BusinessFinanceHandler) GetGrowth(c *gin.Context) {
	filter, err := parseBusinessFinanceReportFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	report, err := h.financeService.GetBusinessFinanceGrowth(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, report)
}

func toCostConfigInput(req businessCostConfigRequest) (service.CostConfigInput, error) {
	input := service.CostConfigInput{
		Code:             req.Code,
		Name:             req.Name,
		Category:         req.Category,
		Amount:           req.Amount,
		Currency:         req.Currency,
		ExchangeRate:     req.ExchangeRate,
		AllocationMethod: req.AllocationMethod,
		Frequency:        req.Frequency,
		Scope:            req.Scope,
		EffectiveTo:      req.EffectiveTo,
		Notes:            req.Notes,
	}
	if req.EffectiveFrom != nil {
		input.EffectiveFrom = req.EffectiveFrom.UTC()
	}
	input.Enabled = req.Enabled == nil || *req.Enabled
	return input, nil
}

func parseBusinessFinanceReportFilter(c *gin.Context) (service.FinanceReportFilter, error) {
	filter := service.FinanceReportFilter{
		Dimension: strings.TrimSpace(strings.ToLower(c.DefaultQuery("dimension", "group"))),
		Model:     strings.TrimSpace(c.Query("model")),
	}
	var err error
	startTime, parseErr := parseFinanceQueryTime(c.Query("start_time"))
	err = parseErr
	if err != nil {
		return filter, fmtFinanceError("Invalid start_time")
	}
	endTime, parseErr := parseFinanceQueryTime(c.Query("end_time"))
	err = parseErr
	if err != nil {
		return filter, fmtFinanceError("Invalid end_time")
	}
	if startTime != nil {
		filter.StartTime = *startTime
	}
	if endTime != nil {
		filter.EndTime = *endTime
	}
	if value := strings.TrimSpace(c.Query("group_id")); value != "" {
		filter.GroupID, err = strconv.ParseInt(value, 10, 64)
		if err != nil || filter.GroupID <= 0 {
			return filter, fmtFinanceError("Invalid group_id")
		}
	}
	if value := strings.TrimSpace(c.Query("channel_id")); value != "" {
		filter.ChannelID, err = strconv.ParseInt(value, 10, 64)
		if err != nil || filter.ChannelID <= 0 {
			return filter, fmtFinanceError("Invalid channel_id")
		}
	}
	if value := strings.TrimSpace(c.Query("min_margin")); value != "" {
		if parsed, parseErr := strconv.ParseFloat(value, 64); parseErr != nil {
			return filter, fmtFinanceError("Invalid min_margin")
		} else {
			filter.MinMargin = parsed
		}
	}
	return filter, nil
}

func toExpenseInput(req businessExpenseRequest) (service.ExpenseInput, error) {
	if req.OccurredAt == nil {
		return service.ExpenseInput{}, fmtFinanceError("occurred_at is required")
	}
	return service.ExpenseInput{
		Category:         req.Category,
		Name:             req.Name,
		Amount:           req.Amount,
		Currency:         req.Currency,
		ExchangeRate:     req.ExchangeRate,
		OccurredAt:       req.OccurredAt.UTC(),
		PeriodStart:      utcTime(req.PeriodStart),
		PeriodEnd:        utcTime(req.PeriodEnd),
		AllocationMethod: req.AllocationMethod,
		Scope:            req.Scope,
		Notes:            req.Notes,
	}, nil
}

func utcTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	t := value.UTC()
	return &t
}

func parseFinanceID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid id")
		return 0, false
	}
	return id, true
}

func parseFinanceQueryTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func financeOperatorID(c *gin.Context) int64 {
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		return subject.UserID
	}
	return 0
}

type financeHandlerError string

func (e financeHandlerError) Error() string { return string(e) }

func fmtFinanceError(message string) error { return financeHandlerError(message) }
