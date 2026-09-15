// Copyright 2026 zhouhouping. All Rights Reserved.

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func forceChangeRouter(flag bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	api.Use(func(c *gin.Context) {
		c.Set(CtxUserID, int64(42))
		c.Next()
	})
	api.Use(ForcePasswordChange(func(_ context.Context, _ int64) bool { return flag }))
	api.GET("/users/profile", func(c *gin.Context) { c.Status(http.StatusOK) })
	api.POST("/auth/change-password", func(c *gin.Context) { c.Status(http.StatusOK) })
	api.POST("/auth/logout", func(c *gin.Context) { c.Status(http.StatusOK) })
	api.GET("/iam/users", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestForcePasswordChangeBlocks(t *testing.T) {
	// 业务接口被拦
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/users", nil)
	w := httptest.NewRecorder()
	forceChangeRouter(true).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("强制改密期间业务接口应 403，得到 %d", w.Code)
	}

	// 白名单放行
	for _, c := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/auth/change-password"},
		{http.MethodPost, "/api/v1/auth/logout"},
		{http.MethodGet, "/api/v1/users/profile"},
	} {
		req := httptest.NewRequest(c.method, c.path, nil)
		w := httptest.NewRecorder()
		forceChangeRouter(true).ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("白名单 %s 应放行，得到 %d", c.path, w.Code)
		}
	}
}

func TestForcePasswordChangeAllowsWhenFlagAbsent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/users", nil)
	w := httptest.NewRecorder()
	forceChangeRouter(false).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("无强制标记应放行，得到 %d", w.Code)
	}
}
