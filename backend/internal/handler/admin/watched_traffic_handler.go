package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type WatchedTrafficHandler struct {
	service *service.WatchedTrafficService
}

func NewWatchedTrafficHandler(svc *service.WatchedTrafficService) *WatchedTrafficHandler {
	return &WatchedTrafficHandler{service: svc}
}

type watchedTrafficConfigRequest struct {
	Enabled *bool    `json:"enabled"`
	UserIDs *[]int64 `json:"user_ids"`
}

func (h *WatchedTrafficHandler) GetConfig(c *gin.Context) {
	cfg, err := h.service.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *WatchedTrafficHandler) UpdateConfig(c *gin.Context) {
	var req watchedTrafficConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cfg, err := h.service.UpdateConfig(c.Request.Context(), service.UpdateWatchedTrafficConfigInput{
		Enabled: req.Enabled,
		UserIDs: req.UserIDs,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *WatchedTrafficHandler) ListLogs(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := service.WatchedTrafficFilter{
		Pagination: pagination.PaginationParams{
			Page:      page,
			PageSize:  pageSize,
			SortOrder: pagination.SortOrderDesc,
		},
	}
	if raw := strings.TrimSpace(c.Query("user_id")); raw != "" {
		userID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || userID <= 0 {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		filter.UserID = &userID
	}
	items, pageResult, err := h.service.ListLogs(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, pageResult.Total, pageResult.Page, pageResult.PageSize)
}
