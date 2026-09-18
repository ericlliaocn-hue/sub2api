package handler

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AcquisitionTouchRequest 前端首次落地发出的信标。
type AcquisitionTouchRequest struct {
	Href     string `json:"href"`
	Referrer string `json:"referrer"`
}

// AcquisitionTouchResponse 返回裁定结果，前端只用来调试。
type AcquisitionTouchResponse struct {
	Class   string `json:"class"`
	Channel string `json:"channel"`
	Level   int    `json:"level"`
	Counted bool   `json:"counted"`
}

// AcquisitionTouch 获客落地信标。
// POST /api/v1/auth/touch
//
// 规则：
//   - 服务端用落地 URL + referrer 跑同一套裁定，得到分类与证据等级。
//   - 已有 Cookie 且本次证据等级没有升级：只回 204，不计访问、不改 Cookie。
//   - 否则按天累加 acquisition_visits_daily，并由服务端 Set-Cookie s2a_touch（30 天）。
func (h *AuthHandler) AcquisitionTouch(c *gin.Context) {
	promotion := h.authService.PromotionService()
	if promotion == nil {
		c.Status(http.StatusNoContent)
		return
	}
	var req AcquisitionTouchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Status(http.StatusNoContent)
		return
	}
	if len(req.Href) > 4096 || len(req.Referrer) > 4096 {
		c.Status(http.StatusNoContent)
		return
	}
	now := time.Now()
	ctx := c.Request.Context()
	touch := service.TouchFromLanding(req.Href, req.Referrer, now)
	siteHost := c.Request.Host
	// referrer 是本站自己：SPA 内刷新或站内跳转，不算外站也不算搜索。
	if touch.ReferrerHost != "" && strings.EqualFold(strings.TrimPrefix(strings.ToLower(touch.ReferrerHost), "www."), normalizeHostForTouch(siteHost)) {
		touch.ReferrerHost = ""
	}
	newLevel := service.TouchLevel(touch, siteHost)

	existing := service.AcquisitionTouchFromContext(ctx)
	hasExisting := !existing.IsZero() || !existing.FirstTouchAt.IsZero()
	if hasExisting {
		existingLevel := service.TouchLevel(existing, siteHost)
		if newLevel <= existingLevel {
			c.Status(http.StatusNoContent)
			return
		}
		// 升级：保留首次时间与空位，用更强证据覆盖。
		touch = touch.Merge(existing)
		touch.FirstTouchAt = now
	}

	result, err := promotion.RecordLandingVisit(ctx, touch, now)
	if err != nil {
		slog.Debug("acquisition touch: record visit failed", "error", err)
	}
	setAcquisitionTouchCookie(c, touch, result.Level)
	c.JSON(http.StatusOK, AcquisitionTouchResponse{Class: result.Class, Channel: result.ChannelCode, Level: result.Level, Counted: err == nil})
}

func setAcquisitionTouchCookie(c *gin.Context, touch service.AcquisitionTouch, level int) {
	value := service.EncodeAcquisitionTouchCookie(touch, level)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     service.AcquisitionTouchCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(service.AcquisitionTouchMaxAge / time.Second),
		Secure:   c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https"),
		HttpOnly: false, // 前端需要读来判断是否升级
		SameSite: http.SameSiteLaxMode,
	})
}

func normalizeHostForTouch(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if idx := strings.IndexByte(host, ':'); idx >= 0 {
		host = host[:idx]
	}
	return strings.TrimPrefix(host, "www.")
}
