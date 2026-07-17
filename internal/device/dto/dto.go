// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

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
	DeviceCode  string  `json:"device_code"`
	EventType   string  `json:"event_type"`
	MaterialCode string `json:"material_code"`
	Qty         float64 `json:"qty"`
	LocationCode string `json:"location_code"`
	Timestamp   int64   `json:"timestamp"`
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
	CreatedAt     string `json:"created_at"`
}

type DeviceListReq struct {
	DeviceCode string `json:"device_code"`
	Type       int    `json:"type"`
	Status     int    `json:"status"`
	Page       int    `json:"page" binding:"min=1"`
	PageSize   int    `json:"page_size" binding:"min=1,max=100"`
}
