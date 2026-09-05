package admin

import (
	"context"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type UpstreamConnectionHandler struct {
	service *service.UpstreamConnectionService
}

func NewUpstreamConnectionHandler(svc *service.UpstreamConnectionService) *UpstreamConnectionHandler {
	return &UpstreamConnectionHandler{service: svc}
}

type upstreamConnectionRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	BaseURL  string `json:"base_url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *UpstreamConnectionHandler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *UpstreamConnectionHandler) Create(c *gin.Context) {
	var req upstreamConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.service.Create(c.Request.Context(), service.UpstreamConnectionInput{Name: strings.TrimSpace(req.Name), Type: strings.TrimSpace(req.Type), BaseURL: strings.TrimSpace(req.BaseURL), Username: strings.TrimSpace(req.Username), Password: req.Password})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, item)
}

func (h *UpstreamConnectionHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid upstream connection ID")
		return
	}
	var req upstreamConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.service.Update(c.Request.Context(), id, service.UpstreamConnectionInput{Name: strings.TrimSpace(req.Name), Type: strings.TrimSpace(req.Type), BaseURL: strings.TrimSpace(req.BaseURL), Username: strings.TrimSpace(req.Username), Password: req.Password})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, item)
}

func (h *UpstreamConnectionHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid upstream connection ID")
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Upstream connection deleted"})
}

func (h *UpstreamConnectionHandler) Test(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid upstream connection ID")
		return
	}
	snapshot, err := h.service.Test(context.WithoutCancel(c.Request.Context()), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, snapshot)
}
