package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type CreationHistoryHandler struct {
	history  *service.CreationHistoryService
	keys     *service.APIKeyService
	settings *service.SettingService
}

func NewCreationHistoryHandler(history *service.CreationHistoryService, keys *service.APIKeyService) *CreationHistoryHandler {
	return &CreationHistoryHandler{history: history, keys: keys}
}

func (h *CreationHistoryHandler) SetSettingService(settings *service.SettingService) {
	h.settings = settings
}

type createCreationTaskRequest struct {
	APIKeyID       int64          `json:"api_key_id" binding:"required"`
	Kind           string         `json:"kind" binding:"required"`
	Model          string         `json:"model" binding:"required"`
	Prompt         string         `json:"prompt" binding:"required"`
	Request        map[string]any `json:"request"`
	IdempotencyKey string         `json:"idempotency_key"`
}

type updateCreationTaskRequest struct {
	Status         string `json:"status"`
	ProviderTaskID string `json:"provider_task_id"`
	ErrorMessage   string `json:"error_message"`
}

type createCreationAssetRequest struct {
	Kind string `json:"kind" binding:"required"`
	URL  string `json:"url" binding:"required"`
}

func (h *CreationHistoryHandler) GetConfig(c *gin.Context) {
	if h.settings == nil {
		response.Error(c, http.StatusServiceUnavailable, "Creation settings service is unavailable")
		return
	}
	settings, err := h.settings.GetCreationSettings(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "Creation settings are unavailable")
		return
	}
	response.Success(c, settings)
}

func (h *CreationHistoryHandler) requireKindEnabled(c *gin.Context, kind string) bool {
	if h.settings == nil {
		response.Error(c, http.StatusServiceUnavailable, "Creation settings service is unavailable")
		return false
	}
	settings, err := h.settings.GetCreationSettings(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "Creation settings are unavailable")
		return false
	}
	if !settings.Enabled {
		response.Forbidden(c, "Creation Studio is disabled")
		return false
	}
	if kind == "image" && !settings.ImageEnabled {
		response.Forbidden(c, "Image generation is disabled")
		return false
	}
	if kind == "video" && !settings.VideoEnabled {
		response.Forbidden(c, "Video generation is disabled")
		return false
	}
	return true
}

func (h *CreationHistoryHandler) CreateTask(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req createCreationTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid creation task request")
		return
	}
	req.Kind = strings.ToLower(strings.TrimSpace(req.Kind))
	req.Model = strings.TrimSpace(req.Model)
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Kind != "image" && req.Kind != "video" || req.Model == "" || req.Prompt == "" {
		response.BadRequest(c, "Invalid creation task request")
		return
	}
	if !h.requireKindEnabled(c, req.Kind) {
		return
	}
	if h.keys == nil {
		response.Error(c, http.StatusServiceUnavailable, "Creation API key service is unavailable")
		return
	}
	key, err := h.keys.GetByID(c.Request.Context(), req.APIKeyID)
	if err != nil || key == nil || key.UserID != subject.UserID {
		response.Forbidden(c, "API key does not belong to the current user")
		return
	}
	if !key.IsActive() || key.IsExpired() || key.IsQuotaExhausted() {
		response.Forbidden(c, "API key is not available")
		return
	}

	task, err := h.history.CreateTask(c.Request.Context(), service.CreateCreationTaskInput{
		UserID:         subject.UserID,
		APIKeyID:       req.APIKeyID,
		Kind:           req.Kind,
		Model:          req.Model,
		Prompt:         req.Prompt,
		Request:        req.Request,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"id": task.ID, "status": task.Status})
}

func (h *CreationHistoryHandler) ListTasks(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	tasks, total, err := h.history.ListTasks(c.Request.Context(), subject.UserID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"items": tasks,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": maxCreationPages(total, pageSize),
		},
	})
}

func (h *CreationHistoryHandler) UpdateTask(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || taskID <= 0 {
		response.BadRequest(c, "Invalid creation task id")
		return
	}
	var req updateCreationTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid creation task update")
		return
	}
	if err := h.history.UpdateTask(c.Request.Context(), subject.UserID, taskID, service.UpdateCreationTaskInput{
		Status:         req.Status,
		ProviderTaskID: req.ProviderTaskID,
		ErrorMessage:   req.ErrorMessage,
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"success": true})
}

func (h *CreationHistoryHandler) AddAsset(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || taskID <= 0 {
		response.BadRequest(c, "Invalid creation task id")
		return
	}
	var asset *service.CreationHistoryAsset
	if strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "multipart/form-data") {
		kind := strings.ToLower(strings.TrimSpace(c.PostForm("kind")))
		file, header, fileErr := c.Request.FormFile("file")
		if fileErr != nil {
			response.BadRequest(c, "Asset file is required")
			return
		}
		defer file.Close()
		asset, err = h.history.SaveUploadedAsset(c.Request.Context(), subject.UserID, taskID, kind, header.Header.Get("Content-Type"), file)
	} else {
		var req createCreationAssetRequest
		if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
			response.BadRequest(c, "Asset URL is required")
			return
		}
		asset, err = h.history.SaveRemoteAsset(c.Request.Context(), subject.UserID, taskID, strings.ToLower(strings.TrimSpace(req.Kind)), req.URL)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, asset)
}

func (h *CreationHistoryHandler) DeleteTask(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || taskID <= 0 {
		response.BadRequest(c, "Invalid creation task id")
		return
	}
	if err := h.history.DeleteTask(c.Request.Context(), subject.UserID, taskID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"success": true})
}

func (h *CreationHistoryHandler) ServeAsset(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, err := strconv.ParseInt(c.Param("asset_id"), 10, 64)
	if err != nil || assetID <= 0 {
		response.BadRequest(c, "Invalid creation asset id")
		return
	}
	path, mimeType, err := h.history.AssetPath(c.Request.Context(), subject.UserID, assetID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "private, max-age=3600")
	c.Header("Content-Disposition", "inline")
	c.Header("Content-Type", mimeType)
	c.File(path)
}

func maxCreationPages(total int64, pageSize int) int64 {
	if pageSize <= 0 || total == 0 {
		return 0
	}
	return (total + int64(pageSize) - 1) / int64(pageSize)
}
