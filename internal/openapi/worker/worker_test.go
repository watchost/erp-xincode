// Copyright 2026 zhouhouping. All Rights Reserved.

package worker

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"erp-system/internal/openapi/model"
)

// ---------- 内存假仓储 ----------

type fakeDeliveryRepo struct {
	mu   sync.Mutex
	list []*model.OpenWebhookDelivery
}

func (f *fakeDeliveryRepo) Create(d *model.OpenWebhookDelivery) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	d.ID = int64(len(f.list) + 1)
	f.list = append(f.list, d)
	return nil
}
func (f *fakeDeliveryRepo) Update(d *model.OpenWebhookDelivery) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, e := range f.list {
		if e.ID == d.ID {
			cp := *d
			f.list[i] = &cp
		}
	}
	return nil
}
func (f *fakeDeliveryRepo) get(id int64) *model.OpenWebhookDelivery {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.list {
		if e.ID == id {
			return e
		}
	}
	return nil
}
func (f *fakeDeliveryRepo) ListDuePending(limit int) ([]model.OpenWebhookDelivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []model.OpenWebhookDelivery
	now := time.Now()
	for _, d := range f.list {
		if d.Status == model.DeliveryStatusPending && !d.NextRetryAt.After(now) {
			out = append(out, *d)
		}
	}
	return out, nil
}
func (f *fakeDeliveryRepo) ListByWebhook(id int64, page, pageSize int) ([]model.OpenWebhookDelivery, int64, error) {
	return nil, 0, nil
}

type fakeWebhookRepo struct {
	hook *model.OpenWebhook
}

func (f *fakeWebhookRepo) Create(h *model.OpenWebhook) error                    { return nil }
func (f *fakeWebhookRepo) FindByID(id int64) (*model.OpenWebhook, error)        { return f.hook, nil }
func (f *fakeWebhookRepo) ListByClientID(c string) ([]model.OpenWebhook, error) { return nil, nil }
func (f *fakeWebhookRepo) ListByEvent(e string) ([]model.OpenWebhook, error)    { return nil, nil }

func newTestWorker(hook *model.OpenWebhook) (*Worker, *fakeDeliveryRepo) {
	deliveries := &fakeDeliveryRepo{}
	w := NewWorker(deliveries, &fakeWebhookRepo{hook: hook})
	w.validateURL = func(string) error { return nil } // 测试投递到 httptest（127.0.0.1）
	return w, deliveries
}

func pendingDelivery(webhookID int64, payload []byte) *model.OpenWebhookDelivery {
	return &model.OpenWebhookDelivery{
		WebhookID:   webhookID,
		Event:       "order.approved",
		Payload:     payload,
		Status:      model.DeliveryStatusPending,
		NextRetryAt: time.Now().Add(-time.Second),
	}
}

// ---------- §5 测试要点 ----------

// webhook 签名验证：投递请求携带 X-ERP-Signature = sha256 HMAC(body, secret)。
func TestDeliverSuccessWithSignature(t *testing.T) {
	payload := []byte(`{"order_no":"PO001"}`)
	secret := "hook-secret"

	var gotSig, gotEvent string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-ERP-Signature")
		gotEvent = r.Header.Get("X-ERP-Event")
		gotBody, _ = io.ReadAll(r.Body)
		rw.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	hook := &model.OpenWebhook{ID: 7, URL: srv.URL, Secret: secret, Status: 1}
	w, deliveries := newTestWorker(hook)
	d := pendingDelivery(7, payload)
	_ = deliveries.Create(d)

	if n := w.ProcessDue(); n != 1 {
		t.Fatalf("应处理 1 条，实际 %d", n)
	}
	got := deliveries.get(d.ID)
	if got.Status != model.DeliveryStatusSuccess || got.DeliveredAt == nil || got.Attempts != 1 {
		t.Fatalf("投递应成功: %+v", got)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if gotSig != want {
		t.Fatalf("签名不匹配: got %q want %q", gotSig, want)
	}
	if gotEvent != "order.approved" || string(gotBody) != string(payload) {
		t.Fatalf("事件头/载荷不符: event=%q body=%q", gotEvent, gotBody)
	}
}

// 投递失败：attempts 累加、进入退避；重试耗尽进死信。
func TestDeliverRetryThenDead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	hook := &model.OpenWebhook{ID: 8, URL: srv.URL, Status: 1}
	w, deliveries := newTestWorker(hook)
	d := pendingDelivery(8, []byte(`{}`))
	_ = deliveries.Create(d)

	// 第 1 次失败：仍为待投递，记录退避时间。
	if n := w.ProcessDue(); n != 1 {
		t.Fatalf("应处理 1 条，实际 %d", n)
	}
	got := deliveries.get(d.ID)
	if got.Status != model.DeliveryStatusPending || got.Attempts != 1 {
		t.Fatalf("失败后应保持待投递: %+v", got)
	}
	if got.NextRetryAt.Before(time.Now()) || got.LastError == "" {
		t.Fatalf("应设置退避时间与错误信息: %+v", got)
	}

	// 推进到重试上限边缘，再次失败 → 死信。
	got.Attempts = model.DeliveryMaxAttempts - 1
	got.NextRetryAt = time.Now().Add(-time.Second)
	if n := w.ProcessDue(); n != 1 {
		t.Fatalf("应处理 1 条，实际 %d", n)
	}
	got = deliveries.get(d.ID) // Update 会替换存储副本，重新取
	if got.Status != model.DeliveryStatusDead || got.LastError == "" {
		t.Fatalf("重试耗尽应进死信: %+v", got)
	}
}

// URL 安全校验失败 → 直接死信（不消耗重试）。
func TestDeliverURLRejected(t *testing.T) {
	hook := &model.OpenWebhook{ID: 9, URL: "http://169.254.169.254/meta", Status: 1}
	w, deliveries := newTestWorker(hook)
	w.validateURL = func(string) error { return errors.New("不允许内网 IP 地址") }
	d := pendingDelivery(9, []byte(`{}`))
	_ = deliveries.Create(d)

	if n := w.ProcessDue(); n != 1 {
		t.Fatalf("应处理 1 条，实际 %d", n)
	}
	if got := deliveries.get(d.ID); got.Status != model.DeliveryStatusDead {
		t.Fatalf("URL 校验失败应死信: %+v", got)
	}
}

// webhook 不存在或停用 → 死信。
func TestDeliverHookMissing(t *testing.T) {
	deliveries := &fakeDeliveryRepo{}
	w := NewWorker(deliveries, &fakeWebhookRepo{hook: nil})
	// FindByID 返回 nil, nil 时 hook.Status 解引用会 panic——确保仓储语义为 record-not-found。
	w.webhookRepo = &missingWebhookRepo{}
	w.validateURL = func(string) error { return nil }
	d := pendingDelivery(404, []byte(`{}`))
	_ = deliveries.Create(d)

	if n := w.ProcessDue(); n != 1 {
		t.Fatalf("应处理 1 条，实际 %d", n)
	}
	if got := deliveries.get(d.ID); got.Status != model.DeliveryStatusDead {
		t.Fatalf("webhook 缺失应死信: %+v", got)
	}
}

type missingWebhookRepo struct{}

func (missingWebhookRepo) Create(h *model.OpenWebhook) error { return nil }
func (missingWebhookRepo) FindByID(id int64) (*model.OpenWebhook, error) {
	return nil, gorm.ErrRecordNotFound
}
func (missingWebhookRepo) ListByClientID(c string) ([]model.OpenWebhook, error) { return nil, nil }
func (missingWebhookRepo) ListByEvent(e string) ([]model.OpenWebhook, error)    { return nil, nil }

// 退避间隔：30s → 1m → 2m → 4m。
func TestRetryBackoff(t *testing.T) {
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute}
	for i, w := range want {
		if got := retryBackoff(i + 1); got != w {
			t.Errorf("retryBackoff(%d)=%v，期望 %v", i+1, got, w)
		}
	}
}
