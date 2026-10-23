// Copyright 2026 zhouhouping. All Rights Reserved.

package model

import (
	"time"
)

type DevDevice struct {
	ID              int64      `json:"id"`
	DeviceCode      string     `json:"device_code"`
	Type            int        `json:"type"`
	Brand           string     `json:"brand"`
	Protocol        string     `json:"protocol"`
	Status          int        `json:"status"`
	LastHeartbeat   time.Time  `json:"last_heartbeat"`
	Config          []byte     `json:"config"`
	APIKey          string     `json:"-"` // WebSocket 接入凭据
	APISecretHash   string     `json:"-" gorm:"column:api_secret_hash"`
	LastIP          string     `json:"last_ip"`
	FirmwareVersion string     `json:"firmware_version"`
	TenantID        int64      `json:"tenant_id"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       *time.Time `json:"-"`
}

// DevDeviceMessage 设备消息流水（上行/下发），下发命令经 WebSocket 推送并等 ack。
type DevDeviceMessage struct {
	ID        int64      `json:"id"`
	MessageID string     `json:"message_id"`
	DeviceID  int64      `json:"device_id"`
	Direction int        `json:"direction"` // 1=上行 2=下发
	MsgType   string     `json:"msg_type"`
	Payload   []byte     `json:"payload" gorm:"type:jsonb"`
	Status    int        `json:"status"` // 0=待处理 1=成功 2=失败
	TenantID  int64      `json:"tenant_id"`
	CreatedAt time.Time  `json:"created_at"`
	AckedAt   *time.Time `json:"acked_at"`
}

type DevDeviceLog struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenant_id"` // P3 §4.2 由租户插件在创建时写入
	DeviceID  int64     `json:"device_id"`
	EventType string    `json:"event_type"`
	Content   []byte    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

const (
	DeviceTypeScanner = 1
	DeviceTypeRFID    = 2
	DeviceTypePDA     = 3
	DeviceTypeWeigh   = 4
)

const (
	DeviceStatusOffline  = 0
	DeviceStatusOnline   = 1
	DeviceStatusMaintain = 2
)

const (
	MessageDirectionUp   = 1 // 设备 → 平台
	MessageDirectionDown = 2 // 平台 → 设备
)

const (
	MessageStatusPending = 0
	MessageStatusSuccess = 1
	MessageStatusFailed  = 2
)

// 消息类型：上行 heartbeat/scan/rfid_read/ack；下发 command/print。
const (
	MsgTypeHeartbeat = "heartbeat"
	MsgTypeScan      = "scan"
	MsgTypeRFIDRead  = "rfid_read"
	MsgTypeAck       = "ack"
	MsgTypeCommand   = "command"
	MsgTypePrint     = "print"
	MsgTypeOnline    = "online"
	MsgTypeOffline   = "offline"
)

func (DevDevice) TableName() string        { return "dev_device" }
func (DevDeviceLog) TableName() string     { return "dev_device_log" }
func (DevDeviceMessage) TableName() string { return "dev_device_message" }
