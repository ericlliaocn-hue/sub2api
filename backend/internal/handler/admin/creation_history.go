package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type CreationHistoryHandler struct {
	history *service.CreationHistoryService
}

func NewCreationHistoryHandler(history *service.CreationHistoryService) *CreationHistoryHandler {
	return &CreationHistoryHandler{history: history}
}

func (h *CreationHistoryHandler) ListTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	tasks, total, err := h.history.ListAdminTasks(c.Request.Context(), page, pageSize, service.CreationAdminHistoryFilters{
		Search: c.Query("search"),
		Kind:   c.Query("kind"),
		Status: c.Query("status"),
		Model:  c.Query("model"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	response.Success(c, gin.H{
		"items": tasks,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": maxCreationAdminPages(total, pageSize),
		},
	})
}

func (h *CreationHistoryHandler) ServeAsset(c *gin.Context) {
	assetID, err := strconv.ParseInt(c.Param("asset_id"), 10, 64)
	if err != nil || assetID <= 0 {
		response.BadRequest(c, "Invalid creation asset id")
		return
	}
	path, mimeType, err := h.history.AdminAssetPath(c.Request.Context(), assetID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "private, max-age=3600")
	c.Header("Content-Disposition", "inline")
	if mimeType != "" {
		c.Header("Content-Type", mimeType)
	}
	c.File(path)
}

func maxCreationAdminPages(total int64, pageSize int) int64 {
	if pageSize <= 0 || total == 0 {
		return 0
	}
	return (total + int64(pageSize) - 1) / int64(pageSize)
}
