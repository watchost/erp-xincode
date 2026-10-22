// Copyright 2026 zhouhouping. All Rights Reserved.

// Package middleware 提供 OpenAPI 对外网关的 Bearer token 校验与 scope 检查。
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	openapiModel "erp-system/internal/openapi/model"
	openapiService "erp-system/internal/openapi/service"
	"erp-system/internal/pkg/response"
)

// gin 上下文键。
const (
	CtxClientID   = "openapi_client_id"
	CtxTokenScope = "openapi_token_scope"
)

// TokenValidator 访问令牌校验抽象（由 OpenAPIService.ValidateAccessToken 满足）。
type TokenValidator func(accessToken string) (*openapiModel.OpenToken, error)

// BearerAuth 校验 Authorization: Bearer <token>，通过后注入 client_id 与 scope。
func BearerAuth(validate TokenValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Err(70005, "缺少 Bearer 访问令牌"))
			return
		}
		token, err := validate(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.FromError(err))
			return
		}
		c.Set(CtxClientID, token.ClientID)
		c.Set(CtxTokenScope, token.Scope)
		c.Next()
	}
}

// RequireScope 要求令牌持有指定 scope（须在 BearerAuth 之后）。
func RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !openapiService.HasScope(c.GetString(CtxTokenScope), scope) {
			c.AbortWithStatusJSON(http.StatusForbidden, response.Err(70007, "令牌缺少所需 scope: "+scope))
			return
		}
		c.Next()
	}
}
