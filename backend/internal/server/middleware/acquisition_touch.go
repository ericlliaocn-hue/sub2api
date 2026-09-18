package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AcquisitionTouch 把前端落地时写的 s2a_touch Cookie 和本次请求的站点 host 放进
// request context。所有自助注册入口（邮箱 / 邮件验证 / 各家 OAuth 完成注册）都在
// /auth 分组下，因此一处中间件就覆盖全部注册链路，不必逐个 handler 传 payload。
func AcquisitionTouch() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := service.WithAcquisitionSiteHost(c.Request.Context(), c.Request.Host)
		if raw, err := c.Cookie(service.AcquisitionTouchCookieName); err == nil && raw != "" {
			if touch, ok := service.ParseAcquisitionTouchCookie(raw); ok {
				ctx = service.WithAcquisitionTouch(ctx, touch)
			}
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
