// Copyright 2026 zhouhouping. All Rights Reserved.

// Package idemp 基于 Idempotency-Key 的服务端去重。
// P0：扫码/写入接口必须带 Idempotency-Key；同一 key 在 TTL 内只处理一次。
// 当前实现只判断"是否重复"（重复返回 ErrDuplicate），不缓存原始响应，
// 响应级重放作为 P1 增强。
package idemp

import (
	"context"
	"errors"
	"time"
)

var ErrDuplicate = errors.New("duplicate idempotency key")

// Store 由 Redis 实现。
type Store interface {
	SetNX(ctx context.Context, key string, value interface{}, expiration int) (bool, error)
}

type Guard struct {
	store Store
	ttl   time.Duration
}

func NewGuard(store Store, ttl time.Duration) *Guard {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Guard{store: store, ttl: ttl}
}

// Acquire 尝试占用 key。返回 nil 表示首次请求；返回 ErrDuplicate 表示重复。
// scope 用于区分不同业务接口，例如 "purchase-inbound"。
func (g *Guard) Acquire(ctx context.Context, scope, key string) error {
	if key == "" {
		// 未带 key 的请求直接放行（可在路由层强制要求，这里不做拦截）
		return nil
	}
	ok, err := g.store.SetNX(ctx, "idemp:"+scope+":"+key, "1", int(g.ttl.Seconds()))
	if err != nil {
		// Redis 故障时保守放行，避免拖垮业务；由 DB 约束兜底
		return nil
	}
	if !ok {
		return ErrDuplicate
	}
	return nil
}
