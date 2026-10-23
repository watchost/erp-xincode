// Copyright 2026 zhouhouping. All Rights Reserved.

package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"erp-system/internal/pkg/logger"
)

// maxAuditBody 审计只保留 4KB 以内的 JSON 请求体，防止大报文撑爆审计表。
const maxAuditBody = 4 * 1024

// AuditEntry 是审计中间件与持久化层之间的传输对象；
// 接口定义在 middleware 包，避免 pkg 反向依赖 iam。
type AuditEntry struct {
	UserID     *int64
	TenantID   int64
	Module     string
	Action     string
	IP         string
	UserAgent  string
	Status     int
	DurationMs int64
	TraceID    string
	ReqParam   json.RawMessage
}

// AuditSink 由 main 装配（闭包适配 auditRepo.Create）。
type AuditSink interface {
	WriteAudit(ctx context.Context, entry AuditEntry)
}

// AuditSinkFunc 让普通函数满足 AuditSink。
type AuditSinkFunc func(ctx context.Context, entry AuditEntry)

func (f AuditSinkFunc) WriteAudit(ctx context.Context, entry AuditEntry) {
	f(ctx, entry)
}

// 递归打码的敏感键（大小写不敏感）。P3 §4.3 补充了密钥、凭证、银行卡类字段。
var sensitiveKeys = map[string]struct{}{
	"password":       {},
	"old_password":   {},
	"new_password":   {},
	"admin_password": {},
	"password_hash":  {},
	"token":          {},
	"access_token":   {},
	"refresh_token":  {},
	"captcha_code":   {},
	"secret":         {},
	"client_secret":  {},
	"api_key":        {},
	"apikey":         {},
	"authorization":  {},
	"private_key":    {},
	"bank_account":   {},
	"card_no":        {},
	"id_card":        {},
}

// auditSummary 请求体未能完整记录时的摘要：保证任何写操作都留下可追溯记录（路径参数 + 查询串 + 体积）。
func auditSummary(c *gin.Context, reason string) json.RawMessage {
	m := map[string]interface{}{
		"_body":         reason,
		"_content_type": c.GetHeader("Content-Type"),
		"_size":         c.Request.ContentLength,
	}
	if len(c.Params) > 0 {
		params := map[string]string{}
		for _, p := range c.Params {
			params[p.Key] = p.Value
		}
		m["_path_params"] = params
	}
	if q := c.Request.URL.RawQuery; q != "" {
		m["_query"] = q
	}
	raw, _ := json.Marshal(maskSensitive(m))
	return raw
}

const maskedValue = "******"

// maskSensitive 对任意 JSON 结构递归打码敏感键，返回可重新序列化的通用结构。
func maskSensitive(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, val := range t {
			if _, ok := sensitiveKeys[strings.ToLower(k)]; ok {
				t[k] = maskedValue
			} else {
				t[k] = maskSensitive(val)
			}
		}
		return t
	case []interface{}:
		for i := range t {
			t[i] = maskSensitive(t[i])
		}
		return t
	default:
		return v
	}
}

// Audit 对 /api/v1 下的写操作（POST/PUT/PATCH/DELETE）异步入审计。
// 登录/刷新/oauth 在 JWT 组外挂载不到本中间件，其审计由 service 层自行写。
func Audit(sink AuditSink) gin.HandlerFunc {
	return func(c *gin.Context) {
		if sink == nil || c.Request.Method == http.MethodGet || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		var bodyRaw json.RawMessage
		ct := c.GetHeader("Content-Type")
		if strings.Contains(ct, "application/json") && c.Request.Body != nil &&
			c.Request.ContentLength > 0 && c.Request.ContentLength <= maxAuditBody {
			peek, readErr := io.ReadAll(io.LimitReader(c.Request.Body, maxAuditBody))
			_ = c.Request.Body.Close()
			// 无论成功与否都要把已读内容回填，保证 handler 能拿到 body
			c.Request.Body = io.NopCloser(bytes.NewBuffer(peek))
			if readErr == nil {
				var generic interface{}
				if json.Unmarshal(peek, &generic) == nil {
					if masked, err := json.Marshal(maskSensitive(generic)); err == nil {
						bodyRaw = masked
					}
				}
			}
		}

		start := time.Now()
		c.Next()
		duration := time.Since(start).Milliseconds()

		// 请求体没有被完整记录（非 JSON / 超过 4KB / 空体 / 解析失败）时记摘要，不留空白记录
		if bodyRaw == nil {
			switch {
			case c.Request.ContentLength == 0:
				bodyRaw = auditSummary(c, "empty")
			case !strings.Contains(ct, "application/json"):
				bodyRaw = auditSummary(c, "not_json")
			case c.Request.ContentLength < 0 || c.Request.ContentLength > maxAuditBody:
				bodyRaw = auditSummary(c, "too_large_or_chunked")
			default:
				bodyRaw = auditSummary(c, "unparsable")
			}
		} else if len(c.Params) > 0 {
			// JSON 体合并路径参数（如 /orders/:order_no/approve），否则无法从审计定位到具体单据
			var generic map[string]interface{}
			if json.Unmarshal(bodyRaw, &generic) == nil {
				params := map[string]string{}
				for _, p := range c.Params {
					params[p.Key] = p.Value
				}
				generic["_path_params"] = params
				if merged, err := json.Marshal(generic); err == nil {
					bodyRaw = merged
				}
			}
		}

		uid := c.GetInt64(CtxUserID)
		entry := AuditEntry{
			TenantID:   c.GetInt64(CtxTenantID),
			Module:     auditModule(c.FullPath()),
			Action:     c.Request.Method + " " + c.FullPath(),
			IP:         c.ClientIP(),
			UserAgent:  c.Request.UserAgent(),
			Status:     c.Writer.Status(),
			DurationMs: duration,
			TraceID:    c.Writer.Header().Get("X-Trace-ID"),
			ReqParam:   bodyRaw,
		}
		if len(entry.UserAgent) > maxUserAgentLen {
			entry.UserAgent = entry.UserAgent[:maxUserAgentLen]
		}
		if uid > 0 {
			entry.UserID = &uid
		}

		// 旁路异步写：脱离请求取消信号，失败只记日志，绝不影响响应。
		go func() {
			ctx := context.WithoutCancel(c.Request.Context())
			defer func() {
				if r := recover(); r != nil {
					logger.Log(ctx).Error("audit sink panic recovered", "error", r)
				}
			}()
			sink.WriteAudit(ctx, entry)
		}()
	}
}

// auditModule 取 /api/v1/<module>/... 的首段作为模块名。
func auditModule(fullPath string) string {
	const prefix = "/api/v1/"
	if !strings.HasPrefix(fullPath, prefix) {
		return ""
	}
	rest := fullPath[len(prefix):]
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		return rest[:i]
	}
	return rest
}
