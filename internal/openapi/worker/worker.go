// Copyright 2026 zhouhouping. All Rights Reserved.

// Package worker 异步投递 webhook：轮询待投递记录，HTTP 推送 + 指数退避重试 + 死信。
package worker

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"erp-system/internal/openapi/model"
	"erp-system/internal/openapi/repository"
	openapiService "erp-system/internal/openapi/service"
)

// errRedirectRejected P1.5 H4：webhook 投递禁止跟随任何重定向。
// 307/308 会携带完整 body 与签名 Header 重发，跟随重定向将绕过
// ValidatePublicURL 的 SSRF 校验（重定向目标未经校验）。
var errRedirectRejected = errors.New("webhook 投递禁止跟随重定向")

// rejectRedirect P1.5 H4：http.Client.CheckRedirect 钩子，一律拒绝。
func rejectRedirect(_ *http.Request, _ []*http.Request) error {
	return errRedirectRejected
}

// secureDialContext P1.5 H4+M28：自定义拨号——先解析 host，逐个校验解析 IP
// 必须为公网（拒绝私网/环回/链路本地/169.254.169.254 等），全部公网才拨号。
// 在拨号时点校验实际连接的 IP，消除「校验时解析」与「拨号时解析」分离造成的
// DNS 重绑定 TOCTOU。
func secureDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("webhook 目标域名解析失败: %w", err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("webhook 目标域名无解析结果: %s", host)
	}
	for _, ip := range ips {
		if !openapiService.IsPublicIP(ip) {
			return nil, fmt.Errorf("webhook 目标解析到内网地址 %s，拒绝拨号", ip)
		}
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, ip := range ips {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// newSecureHTTPClient P1.5 H4：构造投递专用 http.Client——
// 拒绝一切重定向 + 拨号期校验实际解析 IP 为公网。
func newSecureHTTPClient() *http.Client {
	return &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: rejectRedirect,
		Transport: &http.Transport{
			DialContext:           secureDialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		},
	}
}

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
		httpClient:   newSecureHTTPClient(), // P1.5 H4：禁重定向 + 拨号期 IP 校验
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
