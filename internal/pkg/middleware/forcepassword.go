// Copyright 2026 zhouhouping. All Rights Reserved.

package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"erp-system/internal/pkg/response"
)

// ForceChangeChecker 查询用户是否处于必须改密状态（Redis mustchg:<uid>）。
type ForceChangeChecker func(ctx context.Context, userID int64) bool

// 强制改密期间仍允许访问的端点（取 FullPath，含方法精确匹配）。
var forceChangeWhitelist = map[string]struct{}{
	"POST /api/v1/auth/logout":          {},
	"POST /api/v1/auth/change-password": {},
	"GET /api/v1/users/profile":         {},
	"GET /api/v1/users/permissions":     {},
}

// ForcePasswordChange 拦截 mustchg 标记未清除的用户：
// 除改密/登出/个人资料外一律 10009/403，防止重置密码后继续操作业务。
func ForcePasswordChange(checker ForceChangeChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		if checker == nil {
			c.Next()
			return
		}
		userID := c.GetInt64(CtxUserID)
		if userID == 0 || !checker(c.Request.Context(), userID) {
			c.Next()
			return
		}
		if _, ok := forceChangeWhitelist[c.Request.Method+" "+c.FullPath()]; ok {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden,
			response.Err(10009, "密码已过期或被重置，请先修改密码"))
	}
}
