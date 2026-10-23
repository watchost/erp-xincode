// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"context"
	"sync"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"erp-system/internal/openapi/dto"
	"erp-system/internal/openapi/model"
	"erp-system/internal/openapi/repository"
	"erp-system/internal/openapi/service"
	"erp-system/internal/testsupport"
)

// newIntegrationSvc 装配真实 PG 的 OpenAPIService（P1.5 H5/H11 集成断言用）。
func newIntegrationSvc(t *testing.T) (*service.OpenAPIService, *gorm.DB) {
	t.Helper()
	gdb := testsupport.NewDB(t)
	if err := gdb.Exec("TRUNCATE TABLE open_token, open_client RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("清理 open 表失败: %v", err)
	}

	svc := service.NewOpenAPIService(
		repository.NewClientRepository(gdb),
		repository.NewTokenRepository(gdb),
		repository.NewWebhookRepository(gdb),
		repository.NewWebhookDeliveryRepository(gdb),
		repository.NewSyncLogRepository(gdb),
	)

	hash, err := bcrypt.GenerateFromPassword([]byte("it-secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	client := &model.OpenClient{
		ClientID:     "oc_it",
		ClientSecret: string(hash),
		Name:         "集成测试客户端",
		Status:       1,
		Scopes:       []byte(`["read","write"]`),
	}
	if err := gdb.Create(client).Error; err != nil {
		t.Fatalf("创建客户端夹具失败: %v", err)
	}
	return svc, gdb
}

func getIntegrationToken(t *testing.T, svc *service.OpenAPIService) *dto.TokenRes {
	t.Helper()
	res, err := svc.GetToken(context.Background(), dto.TokenReq{
		GrantType: "client_credentials", ClientID: "oc_it", ClientSecret: "it-secret",
	})
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}
	return res
}

// P1.5 H5：签发后 DB 中 token 列为 SHA-256 哈希而非明文；按旧明文查询查不到。
func TestIntegrationTokenStoredAsHash(t *testing.T) {
	svc, gdb := newIntegrationSvc(t)
	res := getIntegrationToken(t, svc)

	var rows []model.OpenToken
	if err := gdb.Where("client_id = ?", "oc_it").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("应只有 1 行 token，实际 %d", len(rows))
	}
	row := rows[0]
	if row.AccessToken == res.AccessToken || row.RefreshToken == res.RefreshToken {
		t.Fatal("DB 中不应存明文 token")
	}
	if row.AccessToken != hashTokenForTest(res.AccessToken) {
		t.Fatalf("access_token 列应为 SHA-256 哈希，实际 %q", row.AccessToken)
	}
	if row.RefreshToken != hashTokenForTest(res.RefreshToken) {
		t.Fatalf("refresh_token 列应为 SHA-256 哈希，实际 %q", row.RefreshToken)
	}

	// 按旧明文查询查不到。
	var cnt int64
	if err := gdb.Model(&model.OpenToken{}).
		Where("access_token = ? OR refresh_token = ?", res.AccessToken, res.RefreshToken).
		Count(&cnt).Error; err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatalf("明文查询应命中 0 行，实际 %d", cnt)
	}

	// 明文 token 仍可正常校验与换发（内部按哈希查询）。
	if _, err := svc.ValidateAccessToken(res.AccessToken); err != nil {
		t.Fatalf("明文 access token 校验应通过: %v", err)
	}
}

// P1.5 H11：同一 refresh token 并发换发恰好一次成功；已旋转的旧 refresh 再用被拒。
func TestIntegrationRefreshConcurrentRotation(t *testing.T) {
	svc, _ := newIntegrationSvc(t)
	first := getIntegrationToken(t, svc)

	const concurrency = 8
	var wg sync.WaitGroup
	errs := make([]error, concurrency)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = svc.RefreshToken(context.Background(), first.RefreshToken)
		}(i)
	}
	wg.Wait()

	success := 0
	for _, err := range errs {
		if err == nil {
			success++
			continue
		}
		if errCode(err) != 70004 {
			t.Fatalf("失败者应报 70004（无效刷新令牌），实际 %v", err)
		}
	}
	if success != 1 {
		t.Fatalf("并发换发应恰好一次成功，实际 %d", success)
	}

	// 旧 refresh 已被消费，再次使用被拒。
	if _, err := svc.RefreshToken(context.Background(), first.RefreshToken); errCode(err) != 70004 {
		t.Fatalf("已旋转的旧 refresh 再用期望 70004，实际 %v", err)
	}
	// 旧 access 一并失效。
	if _, err := svc.ValidateAccessToken(first.AccessToken); errCode(err) != 70005 {
		t.Fatalf("旧 access 应失效（70005），实际 %v", err)
	}
}

// P1.5 H11 串行路径：旋转后新 refresh 可继续换发（链路不断）。
func TestIntegrationRefreshChain(t *testing.T) {
	svc, _ := newIntegrationSvc(t)
	first := getIntegrationToken(t, svc)

	second, err := svc.RefreshToken(context.Background(), first.RefreshToken)
	if err != nil {
		t.Fatalf("首次换发失败: %v", err)
	}
	if _, err := svc.RefreshToken(context.Background(), first.RefreshToken); errCode(err) != 70004 {
		t.Fatalf("旧 refresh 重放期望 70004，实际 %v", err)
	}
	if _, err := svc.RefreshToken(context.Background(), second.RefreshToken); err != nil {
		t.Fatalf("新 refresh 应可继续换发: %v", err)
	}
}
