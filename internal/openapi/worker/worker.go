// Copyright 2026 zhouhouping. All Rights Reserved.

// Package worker 异步投递 webhook：轮询待投递记录，HTTP 推送 + 指数退避重试 + 死信。
package worker

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	"erp-system/internal/openapi/model"
	"erp-system/internal/openapi/repository"
	openapiService "erp-system/internal/openapi/service"
)

// retryBackoff 第 n 次失败后的重试间隔（30s, 1m, 2m, 4m）。
func retryBackoff(attempts int) time.Duration {
	d := 30 * time.Second
	for i := 1; i < attempts; i++ {
		d *= 2
	}
	return d
}

type Worker struct {
	deliveryRepo repository.WebhookDeliveryRepository
	webhookRepo  repository.WebhookRepository
	httpClient   *http.Client
	validateURL  func(string) error // SSRF 校验，测试可注入以指向 httptest
	batchSize    int
	interval     time.Duration
}

func NewWorker(deliveryRepo repository.WebhookDeliveryRepository, webhookRepo repository.WebhookRepository) *Worker {
	return &Worker{
		deliveryRepo: deliveryRepo,
		webhookRepo:  webhookRepo,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		validateURL:  openapiService.ValidatePublicURL,
		batchSize:    50,
		interval:     5 * time.Second,
	}
}

// Run 周期轮询直到 ctx 取消（main 以 goroutine 启动）。
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.ProcessDue()
		}
	}
}

// ProcessDue 处理一批到期投递（每轮返回处理条数，测试可直接调用）。
func (w *Worker) ProcessDue() int {
	list, err := w.deliveryRepo.ListDuePending(w.batchSize)
	if err != nil {
		return 0
	}
	for i := range list {
		w.deliver(&list[i])
	}
	return len(list)
}

func (w *Worker) deliver(d *model.OpenWebhookDelivery) {
	hook, err := w.webhookRepo.FindByID(d.WebhookID)
	if err != nil || hook.Status != 1 {
		w.markDead(d, "webhook 不存在或已停用")
		return
	}

	// 投递前再校验一次 URL（防创建后 DNS 被改为内网地址）。
	if err := w.validateURL(hook.URL); err != nil {
		w.markDead(d, "URL 安全校验失败: "+err.Error())
		return
	}

	req, err := http.NewRequest("POST", hook.URL, bytes.NewReader(d.Payload))
	if err != nil {
		w.markDead(d, "构造请求失败: "+err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ERP-Event", d.Event)
	req.Header.Set("X-ERP-Delivery", fmt.Sprintf("%d", d.ID))
	if hook.Secret != "" {
		mac := hmac.New(sha256.New, []byte(hook.Secret))
		mac.Write(d.Payload)
		req.Header.Set("X-ERP-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}

	resp, err := w.httpClient.Do(req)
	if err == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			now := time.Now()
			d.Status = model.DeliveryStatusSuccess
			d.DeliveredAt = &now
			d.Attempts++
			d.LastError = ""
			_ = w.deliveryRepo.Update(d)
			return
		}
		err = fmt.Errorf("http %d", resp.StatusCode)
	}

	// 失败：重试或死信。
	d.Attempts++
	d.LastError = err.Error()
	if d.Attempts >= model.DeliveryMaxAttempts {
		w.markDead(d, d.LastError)
		return
	}
	d.NextRetryAt = time.Now().Add(retryBackoff(d.Attempts))
	_ = w.deliveryRepo.Update(d)
}

func (w *Worker) markDead(d *model.OpenWebhookDelivery, reason string) {
	d.Status = model.DeliveryStatusDead
	d.LastError = reason
	_ = w.deliveryRepo.Update(d)
}
