// Copyright 2026 zhouhouping. All Rights Reserved.

package auth

import "context"

type ctxKey int

const (
	ctxKeyUserID ctxKey = iota
	ctxKeyUsername
	ctxKeyTenantID
	ctxKeyRoles
	ctxKeyPerms
	ctxKeyJTI
)

// WithUserID 把 user_id 注入 context，供 service 层读取（替代之前硬编码 created_by=1）。
func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, ctxKeyUserID, userID)
}

func UserIDFromContext(ctx context.Context) int64 {
	if v, ok := ctx.Value(ctxKeyUserID).(int64); ok {
		return v
	}
	return 0
}

func WithUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, ctxKeyUsername, username)
}

func UsernameFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyUsername).(string); ok {
		return v
	}
	return ""
}

func WithTenantID(ctx context.Context, tenantID int64) context.Context {
	return context.WithValue(ctx, ctxKeyTenantID, tenantID)
}

func TenantIDFromContext(ctx context.Context) int64 {
	if v, ok := ctx.Value(ctxKeyTenantID).(int64); ok {
		return v
	}
	return 0
}

func WithJTI(ctx context.Context, jti string) context.Context {
	return context.WithValue(ctx, ctxKeyJTI, jti)
}

func JTIFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyJTI).(string); ok {
		return v
	}
	return ""
}
