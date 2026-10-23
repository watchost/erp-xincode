// Copyright 2026 zhouhouping. All Rights Reserved.

package model

import (
	"time"
)

type OpenClient struct {
	ID           int64     `json:"id"`
	TenantID     int64     `json:"tenant_id"` // P3 §4.2 由租户插件在创建时写入
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"-"`
	Name         string    `json:"name"`
	Status       int       `json:"status"`
	Scopes       []byte    `json:"scopes"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// OpenToken P1.5 H5：AccessToken/RefreshToken 列存的是 token 的 SHA-256 哈希（hex），
// 明文只在签发响应中返回，落库与查询一律用哈希。
type OpenToken struct {
	ID           int64     `json:"id"`
	ClientID     string    `json:"client_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Scope        string    `json:"scope"`
	Revoked      int       `json:"revoked"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// OpenWebhookDelivery webhook 投递记录（重试 + 死信）。
type OpenWebhookDelivery struct {
	ID          int64      `json:"id"`
	WebhookID   int64      `json:"webhook_id"`
	Event       string     `json:"event"`
	Payload     []byte     `json:"payload" gorm:"type:jsonb"`
	Status      int        `json:"status"` // 0=待投递 1=成功 2=死信
	Attempts    int        `json:"attempts"`
	NextRetryAt time.Time  `json:"next_retry_at"`
	LastError   string     `json:"last_error"`
	CreatedAt   time.Time  `json:"created_at"`
	DeliveredAt *time.Time `json:"delivered_at"`
}

const (
	DeliveryStatusPending = 0
	DeliveryStatusSuccess = 1
	DeliveryStatusDead    = 2
)

// DeliveryMaxAttempts 重试上限，耗尽进入死信。
const DeliveryMaxAttempts = 5

type OpenWebhook struct {
	ID        int64     `json:"id"`
	ClientID  string    `json:"client_id"`
	Event     string    `json:"event"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type OpenSyncLog struct {
	ID        int64     `json:"id"`
	ClientID  string    `json:"client_id"`
	SyncType  string    `json:"sync_type"`
	SyncData  []byte    `json:"sync_data"`
	Status    int       `json:"status"`
	ErrorMsg  string    `json:"error_msg"`
	CreatedAt time.Time `json:"created_at"`
}

func (OpenClient) TableName() string          { return "open_client" }
func (OpenToken) TableName() string           { return "open_token" }
func (OpenWebhook) TableName() string         { return "open_webhook" }
func (OpenSyncLog) TableName() string         { return "open_sync_log" }
func (OpenWebhookDelivery) TableName() string { return "open_webhook_delivery" }
