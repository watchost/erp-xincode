// Copyright 2026 zhouhouping. All Rights Reserved.

package adapter

import (
	"context"

	"erp-system/internal/device/dto"
)

type DeviceAdapter interface {
	Connect(ctx context.Context) error
	Disconnect() error
	Read(ctx context.Context) (*dto.DeviceScanEvent, error)
	Write(ctx context.Context, data []byte) error
	Heartbeat(ctx context.Context) error
	IsConnected() bool
}

type ScannerAdapter struct {
	deviceCode string
	config     map[string]interface{}
	connected  bool
}

func NewScannerAdapter(deviceCode string, config map[string]interface{}) *ScannerAdapter {
	return &ScannerAdapter{
		deviceCode: deviceCode,
		config:     config,
		connected:  false,
	}
}

func (a *ScannerAdapter) Connect(ctx context.Context) error {
	a.connected = true
	return nil
}

func (a *ScannerAdapter) Disconnect() error {
	a.connected = false
	return nil
}

func (a *ScannerAdapter) Read(ctx context.Context) (*dto.DeviceScanEvent, error) {
	return nil, nil
}

func (a *ScannerAdapter) Write(ctx context.Context, data []byte) error {
	return nil
}

func (a *ScannerAdapter) Heartbeat(ctx context.Context) error {
	return nil
}

func (a *ScannerAdapter) IsConnected() bool {
	return a.connected
}

type RFIDAdapter struct {
	deviceCode string
	config     map[string]interface{}
	connected  bool
}

func NewRFIDAdapter(deviceCode string, config map[string]interface{}) *RFIDAdapter {
	return &RFIDAdapter{
		deviceCode: deviceCode,
		config:     config,
		connected:  false,
	}
}

func (a *RFIDAdapter) Connect(ctx context.Context) error {
	a.connected = true
	return nil
}

func (a *RFIDAdapter) Disconnect() error {
	a.connected = false
	return nil
}

func (a *RFIDAdapter) Read(ctx context.Context) (*dto.DeviceScanEvent, error) {
	return nil, nil
}

func (a *RFIDAdapter) Write(ctx context.Context, data []byte) error {
	return nil
}

func (a *RFIDAdapter) Heartbeat(ctx context.Context) error {
	return nil
}

func (a *RFIDAdapter) IsConnected() bool {
	return a.connected
}

type PDAAdapter struct {
	deviceCode string
	config     map[string]interface{}
	connected  bool
}

func NewPDAAdapter(deviceCode string, config map[string]interface{}) *PDAAdapter {
	return &PDAAdapter{
		deviceCode: deviceCode,
		config:     config,
		connected:  false,
	}
}

func (a *PDAAdapter) Connect(ctx context.Context) error {
	a.connected = true
	return nil
}

func (a *PDAAdapter) Disconnect() error {
	a.connected = false
	return nil
}

func (a *PDAAdapter) Read(ctx context.Context) (*dto.DeviceScanEvent, error) {
	return nil, nil
}

func (a *PDAAdapter) Write(ctx context.Context, data []byte) error {
	return nil
}

func (a *PDAAdapter) Heartbeat(ctx context.Context) error {
	return nil
}

func (a *PDAAdapter) IsConnected() bool {
	return a.connected
}

func CreateAdapter(deviceType int, deviceCode string, config map[string]interface{}) DeviceAdapter {
	switch deviceType {
	case 1:
		return NewScannerAdapter(deviceCode, config)
	case 2:
		return NewRFIDAdapter(deviceCode, config)
	case 3:
		return NewPDAAdapter(deviceCode, config)
	default:
		return NewScannerAdapter(deviceCode, config)
	}
}
