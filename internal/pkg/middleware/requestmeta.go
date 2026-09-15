// Copyright 2026 zhouhouping. All Rights Reserved.

package middleware

import (
	"github.com/gin-gonic/gin"

	"erp-system/internal/pkg/auth"
)

// maxUserAgentLen 与 sys_audit_log.user_agent 列宽对齐。
const maxUserAgentLen = 255

// RequestMeta 把客户端 IP 与 User-Agent 注入标准 context，
// 供登录流（此时 JWTAuth 尚未执行）写审计/登录流水使用。挂在全局中间件链上。
func RequestMeta() gin.HandlerFunc {
	return func(c *gin.Context) {
		ua := c.Request.UserAgent()
		if len(ua) > maxUserAgentLen {
			ua = ua[:maxUserAgentLen]
		}
		ctx := c.Request.Context()
		ctx = auth.WithClientIP(ctx, c.ClientIP())
		ctx = auth.WithUserAgent(ctx, ua)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
