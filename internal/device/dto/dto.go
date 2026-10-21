// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

import (
	"encoding/json"

	"github.com/shopspring/decimal"
)

type DeviceRegisterReq struct {
	DeviceCode string `json:"device_code" binding:"required"`
	Type       int    `json:"type" binding:"required"`
	Brand      string `json:"brand"`
	Protocol   string `json:"protocol" binding:"required"`
	Config     string `json:"config"`
}

type DeviceHeartbeatReq struct {
	DeviceCode string `json:"device_code" binding:"required"`
}

type DeviceScanEvent struct {
	DeviceCode   string          `json:"device_code"`
	EventType    string          `json:"event_type"`
	MaterialCode string          `json:"material_code"`
	Qty          decimal.Decimal `json:"qty"`
	LocationCode string          `json:"location_code"`
	Timestamp    int64           `json:"timestamp"`
}

type DeviceVO struct {
	ID            int64  `json:"id"`
	DeviceCode    string `json:"device_code"`
	Type          int    `json:"type"`
	TypeName      string `json:"type_name"`
	Brand         string `json:"brand"`
	Protocol      string `json:"protocol"`
	Status        int    `json:"status"`
	StatusName    string `json:"status_name"`
	LastHeartbeat string `json:"last_heartbeat"`
	APIKey        string `json:"api_key,omitempty"`
	// APISecret 仅在注册响应中返回一次，服务端只存 SHA256 摘要。
	APISecret string `json:"api_secret,omitempty"`
	CreatedAt string `json:"created_at"`
}

// DeviceCommandReq 下发指令。
type DeviceCommandReq struct {
	MsgType string          `json:"msg_type" binding:"required"` // command/print
	Payload json.RawMessage `json:"payload"`
}

// DeviceCommandVO 指令下发结果。
type DeviceCommandVO struct {
	MessageID string `json:"message_id"`
	Status    int    `json:"status"` // 1=已确认 2=失败/超时
	Acked     bool   `json:"acked"`
}

// DeviceMessageVO 消息流水。
type DeviceMessageVO struct {
	ID        int64           `json:"id"`
	MessageID string          `json:"message_id"`
	Direction int             `json:"direction"`
	MsgType   string          `json:"msg_type"`
	Payload   json.RawMessage `json:"payload"`
	Status    int             `json:"status"`
	CreatedAt string          `json:"created_at"`
	AckedAt   string          `json:"acked_at,omitempty"`
}

type DeviceListReq struct {
	DeviceCode string `form:"device_code" json:"device_code"`
	Type       int    `form:"type" json:"type"`
	Status     int    `form:"status" json:"status"`
	Page       int    `form:"page" json:"page" binding:"min=1"`
	PageSize   int    `form:"page_size" json:"page_size" binding:"min=1,max=100"`
}
