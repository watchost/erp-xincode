// Copyright 2026 zhouhouping. All Rights Reserved.

package routes

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

// buildEngine 用零值处理器装配全部路由：只验证路由是否挂载，不调用处理器，因此不需要数据库。
func buildEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var d Deps
	v := reflect.ValueOf(&d).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() == reflect.Ptr && f.IsNil() && f.CanSet() {
			f.Set(reflect.New(f.Type().Elem()))
		}
	}
	d.JWTSecret = "route-test-secret"
	r := gin.New()
	RegisterRoutes(r, d)
	return r
}

// P3 §4.2–§4.4：api.md §12–§13 列出的接口都必须真正挂载。
// 未带 token 的请求应被 JWT 中间件拦截为 401（说明路由存在）；未挂载的路径才会是 404。
func TestP3RoutesRegistered(t *testing.T) {
	r := buildEngine(t)
	cases := []struct{ method, path string }{
		// 站内消息 §13.3
		{http.MethodGet, "/api/v1/messages"},
		{http.MethodGet, "/api/v1/messages/unread-count"},
		{http.MethodPost, "/api/v1/messages/read-all"},
		{http.MethodPost, "/api/v1/messages/1/read"},
		{http.MethodGet, "/api/v1/messages/subscriptions"},
		{http.MethodPut, "/api/v1/messages/subscriptions"},
		// 租户 §13.1
		{http.MethodGet, "/api/v1/tenant/current"},
		{http.MethodPut, "/api/v1/tenant/current"},
		{http.MethodGet, "/api/v1/admin/tenants"},
		{http.MethodPost, "/api/v1/admin/tenants"},
		{http.MethodPut, "/api/v1/admin/tenants/1"},
		{http.MethodPost, "/api/v1/admin/tenants/1/disable"},
		{http.MethodPost, "/api/v1/admin/tenants/1/enable"},
		{http.MethodGet, "/api/v1/admin/tenants/1/stats"},
		// 审计 §13.2、审批 §12
		{http.MethodGet, "/api/v1/iam/audit-logs"},
		{http.MethodGet, "/api/v1/workflow/tasks/todo"},
		{http.MethodPost, "/api/v1/purchase/orders/PO1/submit"},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(c.method, c.path, nil))
		if w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed {
			t.Errorf("%s %s 未挂载（返回 %d）", c.method, c.path, w.Code)
		} else if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s 应先被 JWT 拦截为 401，实际 %d", c.method, c.path, w.Code)
		}
	}

	// 反向对照：确认测试能区分「未挂载」
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/no-such-route", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("未挂载的路径应返回 404，实际 %d（测试失去区分能力）", w.Code)
	}
}

// Deps 里每个处理器字段都必须至少被一条路由使用，防止再次出现「声明了但没挂路由」。
func TestEveryHandlerDepIsRouted(t *testing.T) {
	r := buildEngine(t)
	used := map[string]bool{}
	for _, rt := range r.Routes() {
		used[rt.Handler] = true
	}
	typ := reflect.TypeOf(Deps{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.Type.Kind() != reflect.Ptr || f.Type.Elem().Kind() != reflect.Struct {
			continue
		}
		pkg := f.Type.Elem().PkgPath()
		found := false
		for h := range used {
			if len(h) > len(pkg) && h[:len(pkg)] == pkg {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Deps.%s（%s）没有任何路由使用", f.Name, f.Type)
		}
	}
}
