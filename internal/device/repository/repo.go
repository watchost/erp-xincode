// Copyright 2026 zhouhouping. All Rights Reserved.

package repository

import (
	"time"

	"erp-system/internal/device/model"
	"gorm.io/gorm"
)

type DeviceRepository interface {
	Create(device *model.DevDevice) error
	Update(device *model.DevDevice) error
	FindByCode(deviceCode string) (*model.DevDevice, error)
	FindByID(id int64) (*model.DevDevice, error)
	FindByAPIKey(apiKey string) (*model.DevDevice, error)
	List(deviceCode string, deviceType, status int, page, pageSize int) ([]model.DevDevice, int64, error)
	MarkOfflineBefore(threshold time.Time) (int64, error)
}

// DeviceMessageRepository 设备消息流水。
type DeviceMessageRepository interface {
	Create(msg *model.DevDeviceMessage) error
	Update(msg *model.DevDeviceMessage) error
	FindByMessageID(messageID string) (*model.DevDeviceMessage, error)
	ListByDevice(deviceID int64, direction int, page, pageSize int) ([]model.DevDeviceMessage, int64, error)
	// MarkAckedIfPending 仅当消息归属 deviceID 且仍处于待确认（status=0）时
	// 推进到终态（成功/失败）。命中返回 true。P1.5 S3+H8：串台/伪造 ack 与
	// 迟到 ack 都不能改写他人消息或翻转已定终态。
	MarkAckedIfPending(messageID string, deviceID int64, status int, ackedAt time.Time) (bool, error)
}

type deviceRepo struct {
	db *gorm.DB
}

func NewDeviceRepository(db *gorm.DB) DeviceRepository {
	return &deviceRepo{db: db}
}

func (r *deviceRepo) Create(device *model.DevDevice) error {
	return r.db.Create(device).Error
}

func (r *deviceRepo) Update(device *model.DevDevice) error {
	return r.db.Save(device).Error
}

func (r *deviceRepo) FindByCode(deviceCode string) (*model.DevDevice, error) {
	var device model.DevDevice
	if err := r.db.Where("device_code = ?", deviceCode).First(&device).Error; err != nil {
		return nil, err
	}
	return &device, nil
}

func (r *deviceRepo) FindByID(id int64) (*model.DevDevice, error) {
	var device model.DevDevice
	if err := r.db.Where("id = ?", id).First(&device).Error; err != nil {
		return nil, err
	}
	return &device, nil
}

func (r *deviceRepo) List(deviceCode string, deviceType, status int, page, pageSize int) ([]model.DevDevice, int64, error) {
	var devices []model.DevDevice
	var total int64
	query := r.db.Model(&model.DevDevice{})
	if deviceCode != "" {
		query = query.Where("device_code LIKE ?", "%"+deviceCode+"%")
	}
	if deviceType > 0 {
		query = query.Where("type = ?", deviceType)
	}
	if status >= 0 {
		query = query.Where("status = ?", status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&devices).Error; err != nil {
		return nil, 0, err
	}
	return devices, total, nil
}

func (r *deviceRepo) FindByAPIKey(apiKey string) (*model.DevDevice, error) {
	var device model.DevDevice
	if err := r.db.Where("api_key = ?", apiKey).First(&device).Error; err != nil {
		return nil, err
	}
	return &device, nil
}

// MarkOfflineBefore 把最后心跳早于 threshold 的在线设备批量置为离线（心跳超时清扫）。
func (r *deviceRepo) MarkOfflineBefore(threshold time.Time) (int64, error) {
	res := r.db.Model(&model.DevDevice{}).
		Where("status = ? AND last_heartbeat < ?", model.DeviceStatusOnline, threshold).
		Updates(map[string]any{"status": model.DeviceStatusOffline, "updated_at": time.Now()})
	return res.RowsAffected, res.Error
}

type deviceMessageRepo struct {
	db *gorm.DB
}

func NewDeviceMessageRepository(db *gorm.DB) DeviceMessageRepository {
	return &deviceMessageRepo{db: db}
}

func (r *deviceMessageRepo) Create(msg *model.DevDeviceMessage) error {
	return r.db.Create(msg).Error
}

func (r *deviceMessageRepo) Update(msg *model.DevDeviceMessage) error {
	return r.db.Save(msg).Error
}

// MarkAckedIfPending 条件更新：归属 + 待确认双守卫，单条 UPDATE 原子完成。
func (r *deviceMessageRepo) MarkAckedIfPending(messageID string, deviceID int64, status int, ackedAt time.Time) (bool, error) {
	res := r.db.Model(&model.DevDeviceMessage{}).
		Where("message_id = ? AND device_id = ? AND status = ?", messageID, deviceID, model.MessageStatusPending).
		Updates(map[string]any{"status": status, "acked_at": ackedAt})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *deviceMessageRepo) FindByMessageID(messageID string) (*model.DevDeviceMessage, error) {
	var msg model.DevDeviceMessage
	if err := r.db.Where("message_id = ?", messageID).First(&msg).Error; err != nil {
		return nil, err
	}
	return &msg, nil
}

func (r *deviceMessageRepo) ListByDevice(deviceID int64, direction int, page, pageSize int) ([]model.DevDeviceMessage, int64, error) {
	var list []model.DevDeviceMessage
	var total int64
	query := r.db.Model(&model.DevDeviceMessage{}).Where("device_id = ?", deviceID)
	if direction > 0 {
		query = query.Where("direction = ?", direction)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
