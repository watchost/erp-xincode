// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"encoding/json"
	"time"

	"erp-system/internal/device/dto"
	"erp-system/internal/device/model"
	"erp-system/internal/device/repository"
	"erp-system/internal/pkg/errors"
)

type DeviceService struct {
	deviceRepo repository.DeviceRepository
}

func NewDeviceService(deviceRepo repository.DeviceRepository) *DeviceService {
	return &DeviceService{deviceRepo: deviceRepo}
}

func (s *DeviceService) Register(ctx context.Context, req dto.DeviceRegisterReq) (*dto.DeviceVO, error) {
	existing, _ := s.deviceRepo.FindByCode(req.DeviceCode)
	if existing != nil {
		return nil, errors.New(60001, 409, "设备编码已存在")
	}

	var configBytes []byte
	if req.Config != "" {
		configBytes = []byte(req.Config)
	}

	device := &model.DevDevice{
		DeviceCode: req.DeviceCode,
		Type:       req.Type,
		Brand:      req.Brand,
		Protocol:   req.Protocol,
		Status:     model.DeviceStatusOnline,
		Config:     configBytes,
	}

	if err := s.deviceRepo.Create(device); err != nil {
		return nil, err
	}

	return s.buildDeviceVO(device), nil
}

func (s *DeviceService) Heartbeat(ctx context.Context, deviceCode string) error {
	device, err := s.deviceRepo.FindByCode(deviceCode)
	if err != nil {
		return errors.New(60002, 404, "设备不存在")
	}

	now := time.Now()
	device.LastHeartbeat = now
	device.Status = model.DeviceStatusOnline

	return s.deviceRepo.Update(device)
}

func (s *DeviceService) List(ctx context.Context, req dto.DeviceListReq) ([]dto.DeviceVO, int64, error) {
	devices, total, err := s.deviceRepo.List(req.DeviceCode, req.Type, req.Status, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}

	var vos []dto.DeviceVO
	for _, d := range devices {
		vos = append(vos, *s.buildDeviceVO(&d))
	}

	return vos, total, nil
}

func (s *DeviceService) GetByCode(ctx context.Context, deviceCode string) (*dto.DeviceVO, error) {
	device, err := s.deviceRepo.FindByCode(deviceCode)
	if err != nil {
		return nil, errors.New(60002, 404, "设备不存在")
	}
	return s.buildDeviceVO(device), nil
}

func (s *DeviceService) buildDeviceVO(d *model.DevDevice) *dto.DeviceVO {
	typeName := ""
	switch d.Type {
	case model.DeviceTypeScanner:
		typeName = "扫码枪"
	case model.DeviceTypeRFID:
		typeName = "RFID读写器"
	case model.DeviceTypePDA:
		typeName = "PDA手持终端"
	case model.DeviceTypeWeigh:
		typeName = "电子秤"
	}

	statusName := ""
	switch d.Status {
	case model.DeviceStatusOffline:
		statusName = "离线"
	case model.DeviceStatusOnline:
		statusName = "在线"
	case model.DeviceStatusMaintain:
		statusName = "维护中"
	}

	return &dto.DeviceVO{
		ID:            d.ID,
		DeviceCode:    d.DeviceCode,
		Type:          d.Type,
		TypeName:      typeName,
		Brand:         d.Brand,
		Protocol:      d.Protocol,
		Status:        d.Status,
		StatusName:    statusName,
		LastHeartbeat: d.LastHeartbeat.Format(time.RFC3339),
		CreatedAt:     d.CreatedAt.Format(time.RFC3339),
	}
}

func (s *DeviceService) ParseScanEvent(data []byte) (*dto.DeviceScanEvent, error) {
	var event dto.DeviceScanEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, errors.New(60003, 400, "无效的扫描事件数据")
	}
	return &event, nil
}
