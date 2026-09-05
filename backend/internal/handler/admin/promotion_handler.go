package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
	"time"
)

type PromotionHandler struct{ service *service.PromotionService }

func NewPromotionHandler(s *service.PromotionService) *PromotionHandler {
	return &PromotionHandler{service: s}
}

type promotionPromoterRequest struct {
	Name                 string  `json:"name"`
	Contact              string  `json:"contact"`
	CommissionRate       float64 `json:"commission_rate"`
	CommissionFreezeDays int     `json:"commission_freeze_days"`
	Enabled              bool    `json:"enabled"`
	Notes                string  `json:"notes"`
}
type promotionChannelRequest struct {
	Code           string   `json:"code"`
	Name           string   `json:"name"`
	ChannelType    string   `json:"channel_type"`
	PromoterID     *int64   `json:"promoter_id"`
	CommissionRate *float64 `json:"commission_rate"`
	Enabled        bool     `json:"enabled"`
	Notes          string   `json:"notes"`
}

func (h *PromotionHandler) ListPromoters(c *gin.Context) {
	x, e := h.service.ListPromoters(c.Request.Context())
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, x)
}
func (h *PromotionHandler) SavePromoter(c *gin.Context) {
	var q promotionPromoterRequest
	if e := c.ShouldBindJSON(&q); e != nil {
		response.BadRequest(c, e.Error())
		return
	}
	in := service.PromotionPromoterInput{Name: q.Name, Contact: q.Contact, CommissionRate: q.CommissionRate, CommissionFreezeDays: q.CommissionFreezeDays, Enabled: q.Enabled, Notes: q.Notes}
	var x *service.PromotionPromoter
	var e error
	if id, _ := strconv.ParseInt(c.Param("id"), 10, 64); id > 0 {
		x, e = h.service.UpdatePromoter(c.Request.Context(), id, in)
	} else {
		x, e = h.service.CreatePromoter(c.Request.Context(), in)
	}
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, x)
}
func (h *PromotionHandler) ListChannels(c *gin.Context) {
	x, e := h.service.ListChannels(c.Request.Context())
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, x)
}
func (h *PromotionHandler) SaveChannel(c *gin.Context) {
	var q promotionChannelRequest
	if e := c.ShouldBindJSON(&q); e != nil {
		response.BadRequest(c, e.Error())
		return
	}
	in := service.PromotionChannelInput{Code: q.Code, Name: q.Name, ChannelType: q.ChannelType, PromoterID: q.PromoterID, CommissionRate: q.CommissionRate, Enabled: q.Enabled, Notes: q.Notes}
	var x *service.PromotionChannel
	var e error
	if id, _ := strconv.ParseInt(c.Param("id"), 10, 64); id > 0 {
		x, e = h.service.UpdateChannel(c.Request.Context(), id, in)
	} else {
		x, e = h.service.CreateChannel(c.Request.Context(), in)
	}
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, x)
}
func (h *PromotionHandler) Report(c *gin.Context) {
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -30)
	if v, e := time.Parse(time.RFC3339, c.Query("start_time")); e == nil {
		start = v
	}
	if v, e := time.Parse(time.RFC3339, c.Query("end_time")); e == nil {
		end = v
	}
	x, e := h.service.Report(c.Request.Context(), start, end, c.Query("mode"))
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, x)
}

func promotionQueryInt64(c *gin.Context, key string) int64 {
	value, _ := strconv.ParseInt(c.Query(key), 10, 64)
	return value
}

func promotionQueryLimit(c *gin.Context) int {
	value, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	return value
}

func (h *PromotionHandler) ListAttributionEvents(c *gin.Context) {
	items, err := h.service.ListAttributionEvents(c.Request.Context(), promotionQueryLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *PromotionHandler) ListCommissions(c *gin.Context) {
	items, err := h.service.ListCommissions(c.Request.Context(), promotionQueryInt64(c, "promoter_id"), c.Query("status"), promotionQueryLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *PromotionHandler) ListSettlements(c *gin.Context) {
	items, err := h.service.ListSettlements(c.Request.Context(), promotionQueryInt64(c, "promoter_id"), promotionQueryLimit(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

type promotionSettlementRequest struct {
	PromoterID int64     `json:"promoter_id"`
	PeriodEnd  time.Time `json:"period_end"`
	Notes      string    `json:"notes"`
}

func (h *PromotionHandler) CreateSettlement(c *gin.Context) {
	var request promotionSettlementRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.service.CreateSettlement(c.Request.Context(), service.PromotionSettlementInput{PromoterID: request.PromoterID, PeriodEnd: request.PeriodEnd, Notes: request.Notes})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

type promotionSettlementStatusRequest struct {
	Status string `json:"status"`
}

func (h *PromotionHandler) UpdateSettlementStatus(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var request promotionSettlementStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.service.UpdateSettlementStatus(c.Request.Context(), id, request.Status)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}
