// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"erp-system/internal/openapi/dto"
	"erp-system/internal/openapi/model"
	"erp-system/internal/openapi/service"
	apperrors "erp-system/internal/pkg/errors"
)

// ---------- 内存假仓储 ----------

type fakeClientRepo struct {
	byID map[string]*model.OpenClient
}

func (f *fakeClientRepo) FindByClientID(clientID string) (*model.OpenClient, error) {
	c, ok := f.byID[clientID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return c, nil
}
func (f *fakeClientRepo) Create(c *model.OpenClient) error { f.byID[c.ClientID] = c; return nil }
func (f *fakeClientRepo) Update(c *model.OpenClient) error { f.byID[c.ClientID] = c; return nil }
func (f *fakeClientRepo) List(page, pageSize int) ([]model.OpenClient, int64, error) {
	out := make([]model.OpenClient, 0, len(f.byID))
	for _, c := range f.byID {
		out = append(out, *c)
	}
	return out, int64(len(out)), nil
}

type fakeTokenRepo struct {
	byAccess  map[string]*model.OpenToken
	byRefresh map[string]*model.OpenToken
}

func newFakeTokenRepo() *fakeTokenRepo {
	return &fakeTokenRepo{byAccess: map[string]*model.OpenToken{}, byRefresh: map[string]*model.OpenToken{}}
}
func (f *fakeTokenRepo) Create(t *model.OpenToken) error {
	f.byAccess[t.AccessToken] = t
	f.byRefresh[t.RefreshToken] = t
	return nil
}
func (f *fakeTokenRepo) FindByAccessToken(at string) (*model.OpenToken, error) {
	t, ok := f.byAccess[at]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return t, nil
}
func (f *fakeTokenRepo) FindByRefreshToken(rt string) (*model.OpenToken, error) {
	t, ok := f.byRefresh[rt]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return t, nil
}
func (f *fakeTokenRepo) DeleteByAccessToken(at string) error {
	if t, ok := f.byAccess[at]; ok {
		delete(f.byRefresh, t.RefreshToken)
		delete(f.byAccess, at)
	}
	return nil
}

type fakeWebhookRepo struct {
	hooks  []*model.OpenWebhook
	nextID int64
}

func (f *fakeWebhookRepo) Create(h *model.OpenWebhook) error {
	f.nextID++
	h.ID = f.nextID
	h.CreatedAt = time.Now()
	f.hooks = append(f.hooks, h)
	return nil
}
func (f *fakeWebhookRepo) FindByID(id int64) (*model.OpenWebhook, error) {
	for _, h := range f.hooks {
		if h.ID == id {
			return h, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeWebhookRepo) ListByClientID(clientID string) ([]model.OpenWebhook, error) {
	var out []model.OpenWebhook
	for _, h := range f.hooks {
		if h.ClientID == clientID {
			out = append(out, *h)
		}
	}
	return out, nil
}
func (f *fakeWebhookRepo) ListByEvent(event string) ([]model.OpenWebhook, error) {
	var out []model.OpenWebhook
	for _, h := range f.hooks {
		if h.Event == event && h.Status == 1 {
			out = append(out, *h)
		}
	}
	return out, nil
}

type fakeDeliveryRepo struct {
	list []*model.OpenWebhookDelivery
}

func (f *fakeDeliveryRepo) Create(d *model.OpenWebhookDelivery) error {
	d.ID = int64(len(f.list) + 1)
	f.list = append(f.list, d)
	return nil
}
func (f *fakeDeliveryRepo) Update(d *model.OpenWebhookDelivery) error { return nil }
func (f *fakeDeliveryRepo) ListDuePending(limit int) ([]model.OpenWebhookDelivery, error) {
	return nil, nil
}
func (f *fakeDeliveryRepo) ListByWebhook(id int64, page, pageSize int) ([]model.OpenWebhookDelivery, int64, error) {
	return nil, 0, nil
}

type fakeSyncLogRepo struct{ n int }

func (f *fakeSyncLogRepo) Create(l *model.OpenSyncLog) error { f.n++; l.ID = int64(f.n); return nil }
func (f *fakeSyncLogRepo) FindByID(id int64) (*model.OpenSyncLog, error) {
	return nil, gorm.ErrRecordNotFound
}

// ---------- 辅助 ----------

func errCode(err error) int {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return 0
}

func newSvc(secret string, status int, scopes []string) (*service.OpenAPIService, *fakeTokenRepo, *fakeDeliveryRepo) {
	hash, _ := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.MinCost)
	scopesJSON, _ := json.Marshal(scopes)
	clients := &fakeClientRepo{byID: map[string]*model.OpenClient{
		"oc_test": {
			ClientID:     "oc_test",
			ClientSecret: string(hash),
			Name:         "test",
			Status:       status,
			Scopes:       scopesJSON,
		},
	}}
	tokens := newFakeTokenRepo()
	deliveries := &fakeDeliveryRepo{}
	svc := service.NewOpenAPIService(clients, tokens, &fakeWebhookRepo{}, deliveries, &fakeSyncLogRepo{})
	return svc, tokens, deliveries
}

// ---------- §5 测试要点 ----------

func TestGetTokenBadSecret(t *testing.T) {
	svc, _, _ := newSvc("s3cret", 1, []string{"read"})
	_, err := svc.GetToken(context.Background(), dto.TokenReq{
		GrantType: "client_credentials", ClientID: "oc_test", ClientSecret: "wrong",
	})
	if errCode(err) != 70002 {
		t.Fatalf("期望 70002，实际 %v", err)
	}
}

func TestGetTokenScopeDenied(t *testing.T) {
	svc, _, _ := newSvc("s3cret", 1, []string{"read"})
	_, err := svc.GetToken(context.Background(), dto.TokenReq{
		GrantType: "client_credentials", ClientID: "oc_test", ClientSecret: "s3cret", Scope: "write",
	})
	if errCode(err) != 70007 {
		t.Fatalf("期望 70007，实际 %v", err)
	}
}

func TestGetTokenSuccess(t *testing.T) {
	svc, _, _ := newSvc("s3cret", 1, []string{"read", "write"})
	res, err := svc.GetToken(context.Background(), dto.TokenReq{
		GrantType: "client_credentials", ClientID: "oc_test", ClientSecret: "s3cret", Scope: "write",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.TokenType != "Bearer" || res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatalf("令牌响应不完整: %+v", res)
	}
	if res.Scope != "write" {
		t.Fatalf("scope 应为申请子集 write，实际 %q", res.Scope)
	}
}

// 空 scope 申请 → 授予客户端全集。
func TestGetTokenDefaultScope(t *testing.T) {
	svc, _, _ := newSvc("s3cret", 1, []string{"read", "write"})
	res, err := svc.GetToken(context.Background(), dto.TokenReq{
		GrantType: "client_credentials", ClientID: "oc_test", ClientSecret: "s3cret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !service.HasScope(res.Scope, "read") || !service.HasScope(res.Scope, "write") {
		t.Fatalf("空申请应授予全集，实际 %q", res.Scope)
	}
}

// 无效/过期 token → 401（70005/70006）。
func TestValidateAccessToken(t *testing.T) {
	svc, tokens, _ := newSvc("s3cret", 1, []string{"read"})

	if _, err := svc.ValidateAccessToken("no-such-token"); errCode(err) != 70005 {
		t.Fatalf("无效 token 期望 70005，实际 %v", err)
	}

	expired := &model.OpenToken{
		ClientID: "oc_test", AccessToken: "at-expired", RefreshToken: "rt-expired",
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	_ = tokens.Create(expired)
	if _, err := svc.ValidateAccessToken("at-expired"); errCode(err) != 70006 {
		t.Fatalf("过期 token 期望 70006，实际 %v", err)
	}

	valid := &model.OpenToken{
		ClientID: "oc_test", AccessToken: "at-ok", RefreshToken: "rt-ok",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	_ = tokens.Create(valid)
	tok, err := svc.ValidateAccessToken("at-ok")
	if err != nil || tok.ClientID != "oc_test" {
		t.Fatalf("有效 token 应通过: %v", err)
	}
}

// refresh 旋转：旧 refresh 一次性作废，复用失败。
func TestRefreshTokenRotation(t *testing.T) {
	svc, _, _ := newSvc("s3cret", 1, []string{"read"})
	first, err := svc.GetToken(context.Background(), dto.TokenReq{
		GrantType: "client_credentials", ClientID: "oc_test", ClientSecret: "s3cret",
	})
	if err != nil {
		t.Fatal(err)
	}

	second, err := svc.RefreshToken(context.Background(), first.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if second.AccessToken == first.AccessToken {
		t.Fatal("旋转后应签发新 access token")
	}
	// 旧 access 一并失效。
	if _, err := svc.ValidateAccessToken(first.AccessToken); errCode(err) != 70005 {
		t.Fatalf("旧 access 应失效，实际 %v", err)
	}
	// 旧 refresh 重放失败。
	if _, err := svc.RefreshToken(context.Background(), first.RefreshToken); errCode(err) != 70004 {
		t.Fatalf("旧 refresh 重放期望 70004，实际 %v", err)
	}
}

// SSRF 防护：内网 URL 被拒。
func TestValidatePublicURL(t *testing.T) {
	bad := []string{
		"http://127.0.0.1/hook",
		"http://localhost/hook",
		"http://10.1.2.3/hook",
		"http://172.16.0.1/hook",
		"http://192.168.1.1/hook",
		"http://169.254.169.254/latest/meta-data",
		"http://[::1]/hook",
		"ftp://8.8.8.8/hook",
		"http://user:pass@8.8.8.8/hook",
		"http://foo.internal/hook",
		"not-a-url",
	}
	for _, u := range bad {
		if err := service.ValidatePublicURL(u); err == nil {
			t.Errorf("应拒绝 %q", u)
		}
	}
	if err := service.ValidatePublicURL("https://8.8.8.8/hook"); err != nil {
		t.Errorf("公网 IP 应放行: %v", err)
	}
}

// 创建 webhook 时内网 URL 被拒（70008）。
func TestCreateWebhookSSRF(t *testing.T) {
	svc, _, _ := newSvc("s3cret", 1, []string{"write"})
	_, err := svc.CreateWebhook(context.Background(), "oc_test", dto.WebhookReq{
		Event: "order.approved", URL: "http://192.168.0.9/hook",
	})
	if errCode(err) != 70008 {
		t.Fatalf("内网 URL 期望 70008，实际 %v", err)
	}
}

// DispatchEvent 只为订阅该事件的启用 webhook 生成待投递记录。
func TestDispatchEvent(t *testing.T) {
	hooks := &fakeWebhookRepo{}
	deliveries := &fakeDeliveryRepo{}
	svc := service.NewOpenAPIService(
		&fakeClientRepo{byID: map[string]*model.OpenClient{}},
		newFakeTokenRepo(), hooks, deliveries, &fakeSyncLogRepo{},
	)
	_ = hooks.Create(&model.OpenWebhook{ClientID: "a", Event: "order.approved", URL: "https://8.8.8.8/x", Status: 1})
	_ = hooks.Create(&model.OpenWebhook{ClientID: "b", Event: "order.approved", URL: "https://8.8.8.8/y", Status: 1})
	_ = hooks.Create(&model.OpenWebhook{ClientID: "c", Event: "order.approved", URL: "https://8.8.8.8/z", Status: 0}) // 停用
	_ = hooks.Create(&model.OpenWebhook{ClientID: "d", Event: "stock.low", URL: "https://8.8.8.8/w", Status: 1})

	n, err := svc.DispatchEvent("order.approved", []byte(`{"order_no":"PO1"}`))
	if err != nil || n != 2 {
		t.Fatalf("应投递 2 条（停用/异事件不计）: n=%d err=%v", n, err)
	}
	if len(deliveries.list) != 2 {
		t.Fatalf("待投递记录数应为 2，实际 %d", len(deliveries.list))
	}
	for _, d := range deliveries.list {
		if d.Status != model.DeliveryStatusPending || d.Event != "order.approved" {
			t.Fatalf("投递记录状态不正确: %+v", d)
		}
	}

	n, err = svc.DispatchEvent("no.subscriber", []byte(`{}`))
	if err != nil || n != 0 {
		t.Fatalf("无订阅应投递 0 条: n=%d err=%v", n, err)
	}
}

func TestHasScope(t *testing.T) {
	if !service.HasScope("read write", "write") {
		t.Fatal("应命中 write")
	}
	if service.HasScope("read", "write") {
		t.Fatal("不应命中 write")
	}
	if service.HasScope("", "read") || service.HasScope("reader", "read") {
		t.Fatal("空前缀/子串不应命中")
	}
}
