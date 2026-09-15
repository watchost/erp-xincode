// Copyright 2026 zhouhouping. All Rights Reserved.

package middleware

import (
	"context"
	stderrors "errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"erp-system/internal/pkg/auth"
	"erp-system/internal/pkg/errors"
	"erp-system/internal/pkg/logger"
	"erp-system/internal/pkg/response"
)

// Context keys for gin.Context store.
const (
	CtxUserID    = "user_id"
	CtxUsername  = "username"
	CtxTenantID  = "tenant_id"
	CtxRoles     = "roles"
	CtxPerms     = "perms"
	CtxJTI       = "jti"
	CtxDeptID    = "dept_id"
	CtxDataScope = "data_scope"
)

// 超级管理员拥有的角色/权限通配符，拥有该角色即跳过权限校验。
const superAdminPerm = "*:*:*"

func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.New().String()
		}
		// 修复：在原 request context 之上叠加 traceID，避免丢 cancel/deadline。
		ctx := logger.SetTraceID(c.Request.Context(), traceID)
		c.Request = c.Request.WithContext(ctx)
		c.Header("X-Trace-ID", traceID)
		c.Next()
	}
}

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.Log(c.Request.Context()).Error("panic recovered", "error", err)
				if !c.Writer.Written() {
					c.JSON(http.StatusInternalServerError, response.Err(10300, "系统内部错误"))
				}
			}
		}()
		c.Next()
	}
}

func CORS(allowOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		for _, allowed := range allowOrigins {
			if allowed == "*" || origin == allowed {
				c.Header("Access-Control-Allow-Origin", origin)
				if allowed != "*" {
					c.Header("Access-Control-Allow-Credentials", "true")
				}
				break
			}
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header(
			"Access-Control-Allow-Headers",
			"Content-Type, Authorization, X-Trace-ID, Idempotency-Key",
		)
		c.Header("Access-Control-Expose-Headers", "X-Trace-ID")
		c.Header("Access-Control-Max-Age", "600")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Timeout 把带 deadline 的 ctx 注入 request，由下游 DB/HTTP 调用感知。
// 不再在 goroutine 中调用 c.Next()，避免重复写响应；仅当 handler 未写任何响应时回 504。
func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		if ctx.Err() == context.DeadlineExceeded && !c.Writer.Written() {
			logger.Log(c.Request.Context()).Warn("request timeout")
			c.JSON(http.StatusGatewayTimeout, response.Err(10300, "请求超时"))
			c.Abort()
		}
	}
}

// BlacklistChecker 在每次请求时判断 claims 是否仍然有效：
// jti 登出黑名单 + 改密时间戳双重判定。由 IAM service 实现（查 Redis）；为 nil 时不校验。
type BlacklistChecker func(ctx context.Context, claims *auth.Claims) bool

// JWTAuth 真正校验 Bearer token 并把用户上下文写入 gin.Context。
// P0 之前的实现只判断 header 非空即放行，属于任意伪造可通过的严重漏洞。
// checker 非空时，会额外校验 token 的 jti 是否在登出黑名单中。
func JWTAuth(secret string, checker BlacklistChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearer(c.GetHeader("Authorization"))
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Err(10001, "未认证"))
			return
		}

		claims, err := auth.ParseToken(secret, token)
		if err != nil {
			var ae *errors.AppError
			code := 10003
			if stderrors.As(err, &ae) {
				code = ae.Code
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Err(code, err.Error()))
			return
		}

		// jti 黑名单 / 改密失效：无效 claims 即使 token 未过期也一律拒绝。
		if checker != nil && !checker(c.Request.Context(), claims) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Err(10003, "登录已失效，请重新登录"))
			return
		}

		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxUsername, claims.Username)
		c.Set(CtxTenantID, claims.TenantID)
		c.Set(CtxRoles, claims.Roles)
		c.Set(CtxPerms, claims.Perms)
		c.Set(CtxJTI, claims.JTI)
		c.Set(CtxDeptID, claims.DeptID)
		c.Set(CtxDataScope, claims.DataScope)

		// 同时写入标准 context，供 service 层读取（如 created_by、数据权限、审计日志）。
		ctx := c.Request.Context()
		ctx = auth.WithUserID(ctx, claims.UserID)
		ctx = auth.WithUsername(ctx, claims.Username)
		ctx = auth.WithTenantID(ctx, claims.TenantID)
		ctx = auth.WithJTI(ctx, claims.JTI)
		ctx = auth.WithDeptID(ctx, claims.DeptID)
		ctx = auth.WithDataScope(ctx, claims.DataScope)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// RequirePermission 校验当前用户是否拥有任一传入权限码。
// 拥有 superAdminPerm (`*:*:*`) 直接放行。
func RequirePermission(codes ...string) gin.HandlerFunc {
	needed := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		if code != "" {
			needed[code] = struct{}{}
		}
	}
	return func(c *gin.Context) {
		perms, _ := c.Get(CtxPerms)
		permList, _ := perms.([]string)
		for _, p := range permList {
			if p == superAdminPerm {
				c.Next()
				return
			}
			if _, ok := needed[p]; ok {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, response.Err(10004, "无权限访问"))
	}
}

// OptionalAuth 与 JWTAuth 相同，但没有 token 时也放行（用于可选登录的接口）。
func OptionalAuth(secret string, checker BlacklistChecker) gin.HandlerFunc {
	jwtAuth := JWTAuth(secret, checker)
	return func(c *gin.Context) {
		if extractBearer(c.GetHeader("Authorization")) == "" {
			c.Next()
			return
		}
		jwtAuth(c)
	}
}

func extractBearer(header string) string {
	if header == "" {
		return ""
	}
	// 接受 "Bearer xxx" 或裸 token
	if strings.HasPrefix(header, "Bearer ") || strings.HasPrefix(header, "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return strings.TrimSpace(header)
}
