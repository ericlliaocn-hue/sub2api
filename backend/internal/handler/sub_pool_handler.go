package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// SubPoolUserHandler exposes the caller's own sub-pool to them.
type SubPoolUserHandler struct {
	subPoolService *service.SubPoolService
}

func NewSubPoolUserHandler(subPoolService *service.SubPoolService) *SubPoolUserHandler {
	return &SubPoolUserHandler{subPoolService: subPoolService}
}

// GetPeerBoard returns how many people share the upstream accounts behind this
// key and, for each of them anonymously, when they started calling, when they
// last called, and how much they called today and over the last 7 days.
//
// GET /api/v1/keys/:id/pool-peers
func (h *SubPoolUserHandler) GetPeerBoard(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	apiKeyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || apiKeyID <= 0 {
		response.BadRequest(c, "Invalid API key ID")
		return
	}

	board, err := h.subPoolService.PeerBoard(c.Request.Context(), subject.UserID, apiKeyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, board)
}
