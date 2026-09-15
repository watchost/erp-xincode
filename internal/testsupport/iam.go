// Copyright 2026 zhouhouping. All Rights Reserved.

package testsupport

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"gorm.io/gorm"

	iamRepository "erp-system/internal/iam/repository"
	iamService "erp-system/internal/iam/service"
	"erp-system/internal/pkg/db"
)

// FakeKV 是 service.KVStore 的进程内实现（map+mutex），TTL 不实际过期
// （需要过期语义的用例直接 Del）。登录防爆破/验证码/强制改密测试共用。
type FakeKV struct {
	mu    sync.Mutex
	store map[string]string
}

func NewFakeKV() *FakeKV {
	return &FakeKV{store: make(map[string]string)}
}

func (f *FakeKV) Set(_ context.Context, key string, value interface{}, _ int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.store[key] = fmt.Sprint(value)
	return nil
}

func (f *FakeKV) Get(_ context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.store[key], nil
}

func (f *FakeKV) Del(_ context.Context, keys ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, k := range keys {
		delete(f.store, k)
	}
	return nil
}

func (f *FakeKV) Exists(_ context.Context, key string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.store[key]
	return ok, nil
}

func (f *FakeKV) Incr(_ context.Context, key string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := int64(1)
	if v, ok := f.store[key]; ok {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			n = parsed + 1
		}
	}
	f.store[key] = strconv.FormatInt(n, 10)
	return n, nil
}

func (f *FakeKV) Expire(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return true, nil
}

// Len 返回当前键数量（测试断言用）。
func (f *FakeKV) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.store)
}

// IAMDeps 聚合 IAM service 与其内部状态抓手。
type IAMDeps struct {
	Service *iamService.IAMService
	KV      *FakeKV
}

// IAMJWTSecret 是测试装配使用的 JWT 密钥（仅测试环境）。
const IAMJWTSecret = "test-jwt-secret-please-change-32bytes"

// NewIAMService 装配一套基于真实 Postgres + 假 KV 的 IAM service。
// JWT secret 仅测试使用；access 30 分钟。
func NewIAMService(gdb *gorm.DB) IAMDeps {
	kv := NewFakeKV()
	svc := iamService.NewIAMService(
		gdb, db.NewTxManager(gdb),
		iamRepository.NewUserRepository(gdb),
		iamRepository.NewRoleRepository(gdb),
		iamRepository.NewPermissionRepository(gdb),
		iamRepository.NewUserRoleRepository(gdb),
		iamRepository.NewDepartmentRepository(gdb),
		iamRepository.NewAuditLogRepository(gdb),
		iamRepository.NewLoginAttemptRepository(gdb),
		IAMJWTSecret,
		30*time.Minute, 24*time.Hour,
		kv,
	)
	return IAMDeps{Service: svc, KV: kv}
}
