// Copyright 2026 zhouhouping. All Rights Reserved.

package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"erp-system/internal/openapi/middleware"
	"erp-system/internal/openapi/model"
)

func setup(validate middleware.TokenValidator, scope string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/openapi/v1", middleware.BearerAuth(validate))
	g.GET("/materials", middleware.RequireScope(scope), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"client_id": c.GetString(middleware.CtxClientID)})
	})
	return r
}

func validToken(scope string) middleware.TokenValidator {
	return func(string) (*model.OpenToken, error) {
		return &model.OpenToken{ClientID: "oc_test", Scope: scope, ExpiresAt: time.Now().Add(time.Hour)}, nil
	}
}

func do(r *gin.Engine, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/openapi/v1/materials", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// 无 token / 非 Bearer → 401。
func TestBearerAuthMissing(t *testing.T) {
	r := setup(validToken("read"), "read")
	if w := do(r, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("无 Authorization 应 401，实际 %d", w.Code)
	}
	if w := do(r, "Basic abc"); w.Code != http.StatusUnauthorized {
		t.Fatalf("非 Bearer 应 401，实际 %d", w.Code)
	}
}

// 无效/过期 token（校验器返回错误）→ 401。
func TestBearerAuthInvalid(t *testing.T) {
	r := setup(func(string) (*model.OpenToken, error) {
		return nil, errors.New("无效的访问令牌")
	}, "read")
	if w := do(r, "Bearer bad-token"); w.Code != http.StatusUnauthorized {
		t.Fatalf("无效 token 应 401，实际 %d", w.Code)
	}
}

// scope 不足 → 403。
func TestRequireScopeDenied(t *testing.T) {
	r := setup(validToken("read"), "write")
	if w := do(r, "Bearer ok"); w.Code != http.StatusForbidden {
		t.Fatalf("scope 不足应 403，实际 %d", w.Code)
	}
}

// scope 满足 → 200，且 client_id 来自 token。
func TestRequireScopePass(t *testing.T) {
	r := setup(validToken("read write"), "read")
	w := do(r, "Bearer ok")
	if w.Code != http.StatusOK {
		t.Fatalf("应 200，实际 %d", w.Code)
	}
	if body := w.Body.String(); body != `{"client_id":"oc_test"}` {
		t.Fatalf("client_id 应来自 token: %s", body)
	}
}
