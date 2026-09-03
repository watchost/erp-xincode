// Copyright 2026 zhouhouping. All Rights Reserved.

package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"erp-system/internal/pkg/logger"
	"erp-system/internal/pkg/response"
)

// RateCounter 抽象 Redis INCR + EXPIRE。
// *redis.RedisClient 与 bizno.Counter 均满足该接口。
type RateCounter interface {
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) (bool, error)
}

// LoginRateLimit 登录限流：按客户端 IP 做固定窗口计数，窗口内最多 maxAttempts 次。
// 用于缓解 admin/admin123 等弱口令的在线爆破（审计 P0-2 纵深防御）。
// Redis 故障时 fail-open（记日志后放行），避免 Redis 抖动导致全员无法登录。
// counter 为 nil 时返回透传中间件，便于本地/测试环境关闭限流。
func LoginRateLimit(counter RateCounter, maxAttempts int64, window time.Duration) gin.HandlerFunc {
	if counter == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		key := fmt.Sprintf("ratelimit:login:%s", c.ClientIP())

		n, err := counter.Incr(ctx, key)
		if err != nil {
			logger.Log(ctx).Warn("login rate limiter redis error, fail-open", "error", err)
			c.Next()
			return
		}
		if n == 1 {
			// 首次计数设置窗口过期，由 Redis 自动回收。
			_, _ = counter.Expire(ctx, key, window)
		}
		if n > maxAttempts {
			c.Header("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
			c.AbortWithStatusJSON(http.StatusTooManyRequests,
				response.Err(10201, "登录尝试过于频繁，请稍后再试"))
			return
		}
		c.Next()
	}
}
