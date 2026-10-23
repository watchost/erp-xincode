// Copyright 2026 zhouhouping. All Rights Reserved.

// Package message 实现 P3 §4.4 站内消息与外部通知渠道。
//   - 站内信始终写入 msg_message（与业务同事务，业务回滚则消息一起回滚）；
//   - 接收人订阅了外部渠道时消息标记为待派发，由 Dispatcher 在事务提交后派发，避免「业务回滚了通知却发出去了」；
//   - 外部渠道（email/dingtalk/wecom/webhook）是接口抽象，未配置时记为 skipped，不伪造发送成功。
package message

import (
	"bytes"
	"context"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"erp-system/internal/pkg/auth"
	"erp-system/internal/pkg/errors"
	"erp-system/internal/pkg/logger"
)

const (
	CategoryApprovalTodo = "approval_todo"
	CategoryStockLow     = "stock_low"
	CategorySystem       = "system"

	DispatchNone    = 0
	DispatchPending = 1
	DispatchDone    = 2
)

type MsgMessage struct {
	ID             int64           `json:"id"`
	TenantID       int64           `json:"tenant_id"`
	ReceiverID     int64           `json:"receiver_id"`
	Category       string          `json:"category"`
	Title          string          `json:"title"`
	Content        string          `json:"content"`
	BizType        string          `json:"biz_type"`
	BizNo          string          `json:"biz_no"`
	Link           string          `json:"link"`
	IsRead         bool            `json:"is_read"`
	ReadAt         *time.Time      `json:"read_at"`
	DispatchStatus int16            `json:"dispatch_status"`
	ChannelResult  *json.RawMessage `gorm:"type:jsonb" json:"channel_result"` // 指针：未派发时写 NULL（空字节串不是合法 JSONB）
	CreatedAt      time.Time        `json:"created_at"`
}

func (MsgMessage) TableName() string { return "msg_message" }

type MsgSubscription struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenant_id"`
	UserID    int64     `json:"user_id"`
	Category  string    `json:"category"`
	Channel   string    `json:"channel"`
	Target    string    `json:"target"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (MsgSubscription) TableName() string { return "msg_subscription" }

// Event 一次通知事件；同一事件可发给多个接收人。
// TenantID：消息所属租户。请求上下文带租户时由租户插件覆盖为上下文租户；后台任务按此值写入。
type Event struct {
	TenantID  int64
	Category  string
	Receivers []int64
	Title     string
	Content   string
	BizType   string
	BizNo     string
	Link      string
}

// Channel 外部渠道抽象。
type Channel interface {
	Name() string
	// Send 返回 nil 表示已发出；ErrNotConfigured 表示跳过（未配置凭证/目标）。
	Send(ctx context.Context, target string, m *MsgMessage) error
}

var ErrNotConfigured = stdErrors.New("not configured")

var validChannels = map[string]bool{"email": true, "dingtalk": true, "wecom": true, "webhook": true}
var validCategories = map[string]bool{CategoryApprovalTodo: true, CategoryStockLow: true, CategorySystem: true}

type Service struct {
	db       *gorm.DB
	channels map[string]Channel
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db, channels: map[string]Channel{
		"email":    EmailChannel{},
		"dingtalk": robotChannel{name: "dingtalk", build: dingtalkBody},
		"wecom":    robotChannel{name: "wecom", build: wecomBody},
		"webhook":  robotChannel{name: "webhook", build: webhookBody},
	}}
}

// SetChannel 覆盖渠道实现（配置 SMTP 后可替换 email；测试中注入假渠道）。
func (s *Service) SetChannel(c Channel) { s.channels[c.Name()] = c }

// NotifyTx 在调用方事务内写站内信。接收人去重；没有接收人时不做任何事。
func (s *Service) NotifyTx(ctx context.Context, tx *gorm.DB, ev Event) error {
	if len(ev.Receivers) == 0 {
		return nil
	}
	if !validCategories[ev.Category] {
		return fmt.Errorf("message: 未知消息类别 %q", ev.Category)
	}
	seen := map[int64]bool{}
	for _, uid := range ev.Receivers {
		if uid <= 0 || seen[uid] {
			continue
		}
		seen[uid] = true
		var subs int64
		if err := tx.Model(&MsgSubscription{}).Where("tenant_id = ? AND user_id = ? AND category = ? AND enabled", ev.TenantID, uid, ev.Category).
			Count(&subs).Error; err != nil {
			return err
		}
		status := int16(DispatchNone)
		if subs > 0 {
			status = DispatchPending
		}
		if err := tx.Create(&MsgMessage{
			TenantID: ev.TenantID, ReceiverID: uid, Category: ev.Category, Title: ev.Title, Content: ev.Content,
			BizType: ev.BizType, BizNo: ev.BizNo, Link: ev.Link, DispatchStatus: status, CreatedAt: time.Now(),
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

// Notify 独立事务版本。
func (s *Service) Notify(ctx context.Context, ev Event) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return s.NotifyTx(ctx, tx, ev) })
}

// ---- 收件箱 ----

// mine 显式按租户 + 接收人过滤（不依赖插件），保证收件箱不会跨租户、跨用户。
func (s *Service) mine(ctx context.Context) *gorm.DB {
	tid, _ := auth.TenantFromContext(ctx)
	return s.db.WithContext(ctx).Model(&MsgMessage{}).
		Where("msg_message.tenant_id = ? AND msg_message.receiver_id = ?", tid, auth.UserIDFromContext(ctx))
}

func (s *Service) List(ctx context.Context, category string, unreadOnly bool, page, size int) ([]MsgMessage, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	q := s.mine(ctx)
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if unreadOnly {
		q = q.Where("NOT is_read")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []MsgMessage
	err := q.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// UnreadCount 返回总未读数与按类别分组的未读数（通知铃铛用）。
func (s *Service) UnreadCount(ctx context.Context) (int64, map[string]int64, error) {
	var rows []struct {
		Category string
		N        int64
	}
	if err := s.mine(ctx).Where("NOT is_read").Select("category, COUNT(*) AS n").Group("category").Scan(&rows).Error; err != nil {
		return 0, nil, err
	}
	by := map[string]int64{}
	var total int64
	for _, r := range rows {
		by[r.Category] = r.N
		total += r.N
	}
	return total, by, nil
}

func (s *Service) MarkRead(ctx context.Context, id int64) error {
	res := s.mine(ctx).Where("id = ?", id).Updates(map[string]any{"is_read": true, "read_at": time.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New(82001, 404, "消息不存在")
	}
	return nil
}

func (s *Service) MarkAllRead(ctx context.Context) (int64, error) {
	res := s.mine(ctx).Where("NOT is_read").Updates(map[string]any{"is_read": true, "read_at": time.Now()})
	return res.RowsAffected, res.Error
}

// ---- 订阅 ----

type SubscriptionReq struct {
	Category string `json:"category" binding:"required"`
	Channel  string `json:"channel" binding:"required"`
	Target   string `json:"target" binding:"max=500"`
	Enabled  bool   `json:"enabled"`
}

func (s *Service) ListSubscriptions(ctx context.Context) ([]MsgSubscription, error) {
	tid, _ := auth.TenantFromContext(ctx)
	var list []MsgSubscription
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND user_id = ?", tid, auth.UserIDFromContext(ctx)).
		Order("category, channel").Find(&list).Error
	return list, err
}

// UpsertSubscription 按 (用户, 类别, 渠道) 新增或更新。webhook/dingtalk/wecom 的 target 必须是 https URL。
func (s *Service) UpsertSubscription(ctx context.Context, req SubscriptionReq) error {
	if !validCategories[req.Category] {
		return errors.New(82002, 400, "未知消息类别: "+req.Category)
	}
	if !validChannels[req.Channel] {
		return errors.New(82003, 400, "不支持的渠道: "+req.Channel+"（站内信无需订阅）")
	}
	target := strings.TrimSpace(req.Target)
	if req.Enabled && req.Channel != "email" && !strings.HasPrefix(target, "https://") {
		return errors.New(82004, 400, "机器人/Webhook 地址必须是 https:// URL")
	}
	if req.Enabled && req.Channel == "email" && !strings.Contains(target, "@") {
		return errors.New(82004, 400, "邮箱地址不合法")
	}
	tid, _ := auth.TenantFromContext(ctx)
	sub := &MsgSubscription{TenantID: tid, UserID: auth.UserIDFromContext(ctx), Category: req.Category,
		Channel: req.Channel, Target: target, Enabled: req.Enabled, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}, {Name: "category"}, {Name: "channel"}},
		DoUpdates: clause.AssignmentColumns([]string{"target", "enabled", "updated_at"}),
	}).Create(sub).Error
}

// ---- 派发 ----

// DispatchOnce 处理一批待派发消息（跨租户后台任务）。返回处理条数。
// FOR UPDATE SKIP LOCKED 保证多实例部署时不会重复派发。
func (s *Service) DispatchOnce(ctx context.Context, limit int) (int, error) {
	ctx = auth.WithSkipTenant(ctx)
	done := 0
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var msgs []MsgMessage
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("dispatch_status = ?", DispatchPending).Order("id").Limit(limit).Find(&msgs).Error; err != nil {
			return err
		}
		for i := range msgs {
			m := &msgs[i]
			var subs []MsgSubscription
			if err := tx.Where("tenant_id = ? AND user_id = ? AND category = ? AND enabled", m.TenantID, m.ReceiverID, m.Category).
				Find(&subs).Error; err != nil {
				return err
			}
			result := map[string]string{}
			for _, sub := range subs {
				ch, ok := s.channels[sub.Channel]
				if !ok {
					result[sub.Channel] = "skipped: 渠道未实现"
					continue
				}
				sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				err := ch.Send(sendCtx, sub.Target, m)
				cancel()
				switch {
				case err == nil:
					result[sub.Channel] = "sent"
				case stdErrors.Is(err, ErrNotConfigured):
					result[sub.Channel] = "skipped: " + err.Error()
				default:
					result[sub.Channel] = "failed: " + err.Error()
					logger.Log(ctx).Warn("message channel send failed", "channel", sub.Channel, "msg_id", m.ID, "error", err)
				}
			}
			raw, _ := json.Marshal(result)
			if err := tx.Model(m).Updates(map[string]any{"dispatch_status": DispatchDone, "channel_result": raw}).Error; err != nil {
				return err
			}
			done++
		}
		return nil
	})
	return done, err
}

// RunDispatcher 周期派发，直到 ctx 取消。
func (s *Service) RunDispatcher(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := s.DispatchOnce(ctx, 50); err != nil {
				logger.Log(ctx).Warn("message dispatch failed", "error", err)
			}
		}
	}
}

// ---- 渠道实现 ----

// EmailChannel 未接入 SMTP：始终返回未配置（不伪造发送成功）。
type EmailChannel struct{}

func (EmailChannel) Name() string { return "email" }
func (EmailChannel) Send(context.Context, string, *MsgMessage) error {
	return fmt.Errorf("%w: 未配置 SMTP 发信服务", ErrNotConfigured)
}

// robotChannel 钉钉/企业微信群机器人与通用 Webhook：向订阅里的 https URL POST JSON。
// 说明：未实现钉钉加签（secret）；需要加签的机器人会返回失败并记录在 channel_result。
type robotChannel struct {
	name  string
	build func(m *MsgMessage) any
}

func (r robotChannel) Name() string { return r.name }

var httpClient = &http.Client{Timeout: 5 * time.Second}

func (r robotChannel) Send(ctx context.Context, target string, m *MsgMessage) error {
	if !strings.HasPrefix(target, "https://") {
		return fmt.Errorf("%w: 未配置 https 地址", ErrNotConfigured)
	}
	body, _ := json.Marshal(r.build(m))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	// 钉钉/企业微信即使出错也返回 200，需要看 errcode
	var rr struct {
		ErrCode *int   `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if json.NewDecoder(resp.Body).Decode(&rr) == nil && rr.ErrCode != nil && *rr.ErrCode != 0 {
		return fmt.Errorf("errcode=%d %s", *rr.ErrCode, rr.ErrMsg)
	}
	return nil
}

func text(m *MsgMessage) string {
	if m.Content == "" {
		return m.Title
	}
	return m.Title + "\n" + m.Content
}

func dingtalkBody(m *MsgMessage) any {
	return map[string]any{"msgtype": "text", "text": map[string]string{"content": text(m)}}
}

func wecomBody(m *MsgMessage) any {
	return map[string]any{"msgtype": "text", "text": map[string]string{"content": text(m)}}
}

func webhookBody(m *MsgMessage) any {
	return map[string]any{"id": m.ID, "tenant_id": m.TenantID, "receiver_id": m.ReceiverID, "category": m.Category,
		"title": m.Title, "content": m.Content, "biz_type": m.BizType, "biz_no": m.BizNo, "link": m.Link,
		"created_at": m.CreatedAt}
}
