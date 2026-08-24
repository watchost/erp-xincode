// Copyright 2026 zhouhouping. All Rights Reserved.

// Package bizno generates business document numbers (PO20260814000001 etc.).
// P0 整改：之前用 time.Now().Unix() 作为单号，秒级并发生成必然重复。
// 这里使用 Redis INCR 按天计数，保证同前缀同日单调递增；Redis 不可用时退化为
// 「前缀+yyyyMMddHHmmss+4位随机」，仍保持可接受的碰撞概率。
package bizno

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

// Counter 抽象 Redis INCR + EXPIRE。*redis.Client 直接满足。
type Counter interface {
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) (bool, error)
}

type Generator struct {
	counter Counter
	// fallback 是 Redis 故障时的进程内计数器，降低碰撞概率。
	fallback int64
}

func NewGenerator(c Counter) *Generator {
	return &Generator{counter: c}
}

// Next 返回形如 PO20260814-000001 的单号。prefix 通常为 PO/IB/OB/WO 等。
func (g *Generator) Next(ctx context.Context, prefix string) string {
	now := time.Now()
	day := now.Format("20060102")
	if g.counter != nil {
		key := fmt.Sprintf("bizno:%s:%s", prefix, day)
		seq, err := g.counter.Incr(ctx, key)
		if err == nil {
			// 第一次计数时设置过期，48 小时后自动清理
			if seq == 1 {
				_, _ = g.counter.Expire(ctx, key, 48*time.Hour)
			}
			return fmt.Sprintf("%s%s-%06d", prefix, day, seq)
		}
	}
	// Fallback：秒级时间戳 + 进程计数 + 2 字节随机
	n := atomic.AddInt64(&g.fallback, 1)
	rnd := make([]byte, 2)
	_, _ = rand.Read(rnd)
	return fmt.Sprintf("%s%s-%04d%s", prefix, now.Format("20060102150405"), n%10000, hex.EncodeToString(rnd))
}
