// Copyright 2026 zhouhouping. All Rights Reserved.

package repository

import (
	"gorm.io/gorm"
	"erp-system/internal/device/model"
)

type DeviceRepository interface {
	Create(device *model.DevDevice) error
	Update(device *model.DevDevice) error
	FindByCode(deviceCode string) (*model.DevDevice, error)
	FindByID(id int64) (*model.DevDevice, error)
	List(deviceCode string, deviceType, status int, page, pageSize int) ([]model.DevDevice, int64, error)
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
