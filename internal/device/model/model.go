// Copyright 2026 zhouhouping. All Rights Reserved.

package model

import (
	"time"
)

type DevDevice struct {
	ID            int64     `json:"id"`
	DeviceCode    string    `json:"device_code"`
	Type          int       `json:"type"`
	Brand         string    `json:"brand"`
	Protocol      string    `json:"protocol"`
	Status        int       `json:"status"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	Config        []byte    `json:"config"`
	CreatedAt     time.Time `json:"created_at"`
}

type DevDeviceLog struct {
	ID          int64     `json:"id"`
	DeviceID    int64     `json:"device_id"`
	EventType   string    `json:"event_type"`
	Content     []byte    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
}

const (
	DeviceTypeScanner   = 1
	DeviceTypeRFID      = 2
	DeviceTypePDA       = 3
	DeviceTypeWeigh     = 4
)

const (
	DeviceStatusOffline    = 0
	DeviceStatusOnline     = 1
	DeviceStatusMaintain   = 2
)

func (DevDevice) TableName() string { return "dev_device" }
func (DevDeviceLog) TableName() string { return "dev_device_log" }
