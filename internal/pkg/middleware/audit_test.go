// Copyright 2026 zhouhouping. All Rights Reserved.

package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type captureSink struct {
	ch chan AuditEntry
}

func (s *captureSink) WriteAudit(_ context.Context, e AuditEntry) {
	s.ch <- e
}

func TestAuditWritesMaskedEntry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sink := &captureSink{ch: make(chan AuditEntry, 4)}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(CtxUserID, int64(7))
		c.Set(CtxTenantID, int64(1))
		c.Next()
	})
	r.Use(Audit(sink))
	r.POST("/api/v1/iam/users", func(c *gin.Context) { c.Status(http.StatusCreated) })
	r.GET("/api/v1/iam/users", func(c *gin.Context) { c.Status(http.StatusOK) })

	body := `{"username":"alice","password":"secret123","profile":{"old_password":"old","note":"keep"},` +
		`"tokens":[{"refresh_token":"r-tok"}],"age":30}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("状态码期望 201，得到 %d", w.Code)
	}

	// handler 必须拿到完整 body（审计中间件只 peek 不消费）
	// —— 通过 201 已间接验证；这里再显式断言审计内容。
	select {
	case e := <-sink.ch:
		if e.Status != http.StatusCreated {
			t.Fatalf("审计状态期望 201，得到 %d", e.Status)
		}
		if e.Module != "iam" || e.Action != "POST /api/v1/iam/users" {
			t.Fatalf("模块/动作不正确: %s %s", e.Module, e.Action)
		}
		if e.UserID == nil || *e.UserID != 7 {
			t.Fatalf("用户ID期望 7，得到 %v", e.UserID)
		}
		if e.DurationMs < 0 {
			t.Fatal("耗时不应为负")
		}
		var got map[string]interface{}
		if err := json.Unmarshal(e.ReqParam, &got); err != nil {
			t.Fatalf("审计参数不是合法 JSON: %v", err)
		}
		if got["username"] != "alice" || got["age"].(float64) != 30 {
			t.Fatalf("非敏感字段被破坏: %+v", got)
		}
		if got["password"] != maskedValue {
			t.Fatalf("password 未打码: %v", got["password"])
		}
		profile := got["profile"].(map[string]interface{})
		if profile["old_password"] != maskedValue || profile["note"] != "keep" {
			t.Fatalf("嵌套敏感字段未打码: %+v", profile)
		}
		toks := got["tokens"].([]interface{})[0].(map[string]interface{})
		if toks["refresh_token"] != maskedValue {
			t.Fatalf("数组内敏感字段未打码: %+v", toks)
		}
	case <-time.After(time.Second):
		t.Fatal("未收到异步审计记录")
	}

	// GET 不审计
	req = httptest.NewRequest(http.MethodGet, "/api/v1/iam/users", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)
	select {
	case e := <-sink.ch:
		t.Fatalf("GET 不应写审计，却收到 %+v", e)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAuditNilSinkSkips(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Audit(nil))
	r.POST("/api/v1/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodPost, "/api/v1/x", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req) // 不应 panic
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，得到 %d", w.Code)
	}
}

// P3 §4.3：非 JSON 请求体也要留下可追溯摘要（路径参数 + 查询串 + 体积）。
func TestAuditSummaryForNonJSONBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sink := &captureSink{ch: make(chan AuditEntry, 4)}
	r := gin.New()
	r.Use(Audit(sink))
	r.POST("/api/v1/mdm/materials-import", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/api/v1/purchase/orders/:order_no/approve", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mdm/materials-import?dry=1", bytes.NewBufferString("a,b\n1,2"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	r.ServeHTTP(httptest.NewRecorder(), req)
	e := <-sink.ch
	var got map[string]interface{}
	if err := json.Unmarshal(e.ReqParam, &got); err != nil {
		t.Fatalf("摘要不是合法 JSON: %v", err)
	}
	if got["_body"] != "not_json" || got["_query"] != "dry=1" {
		t.Fatalf("非 JSON 请求应记摘要，实际 %+v", got)
	}

	// 空体 + 路径参数：能定位到具体单据
	req = httptest.NewRequest(http.MethodPost, "/api/v1/purchase/orders/PO001/approve", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)
	e = <-sink.ch
	got = map[string]interface{}{}
	_ = json.Unmarshal(e.ReqParam, &got)
	pp, _ := got["_path_params"].(map[string]interface{})
	if got["_body"] != "empty" || pp["order_no"] != "PO001" {
		t.Fatalf("空体写操作应记录路径参数，实际 %+v", got)
	}
}

// P3 §4.3：新增敏感键（密钥/凭证/银行卡）脱敏，JSON 体合并路径参数。
func TestAuditMasksNewSensitiveKeysAndMergesPathParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sink := &captureSink{ch: make(chan AuditEntry, 4)}
	r := gin.New()
	r.Use(Audit(sink))
	r.PUT("/api/v1/openapi/clients/:id", func(c *gin.Context) { c.Status(http.StatusOK) })

	body := `{"name":"erp","client_secret":"s3cr3t","api_key":"k","bank_account":"6222","admin_password":"p","nested":{"Authorization":"Bearer x"}}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/openapi/clients/42", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)
	e := <-sink.ch
	var got map[string]interface{}
	if err := json.Unmarshal(e.ReqParam, &got); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"client_secret", "api_key", "bank_account", "admin_password"} {
		if got[k] != maskedValue {
			t.Errorf("%s 应脱敏，实际 %v", k, got[k])
		}
	}
	if n, _ := got["nested"].(map[string]interface{}); n["Authorization"] != maskedValue {
		t.Errorf("嵌套 Authorization 应脱敏（大小写不敏感），实际 %v", n["Authorization"])
	}
	if got["name"] != "erp" {
		t.Errorf("非敏感字段被破坏: %v", got["name"])
	}
	if pp, _ := got["_path_params"].(map[string]interface{}); pp["id"] != "42" {
		t.Errorf("应合并路径参数 id=42，实际 %+v", got["_path_params"])
	}
}
