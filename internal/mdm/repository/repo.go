// Copyright 2026 zhouhouping. All Rights Reserved.

package repository

import (
	"gorm.io/gorm"
	"erp-system/internal/mdm/model"
)

type MaterialRepository interface {
	FindByCode(code string) (*model.MdmMaterial, error)
	FindByID(id int64) (*model.MdmMaterial, error)
	Create(material *model.MdmMaterial) error
	Update(material *model.MdmMaterial) error
	List(code, name string, page, pageSize int) ([]model.MdmMaterial, int64, error)
}

type SupplierRepository interface {
	FindByCode(code string) (*model.MdmSupplier, error)
	FindByID(id int64) (*model.MdmSupplier, error)
	Create(supplier *model.MdmSupplier) error
	Update(supplier *model.MdmSupplier) error
	List(code, name string, page, pageSize int) ([]model.MdmSupplier, int64, error)
}

type WarehouseRepository interface {
	FindByCode(code string) (*model.MdmWarehouse, error)
	FindByID(id int64) (*model.MdmWarehouse, error)
	List() ([]model.MdmWarehouse, error)
}

type LocationRepository interface {
	FindByCode(warehouseID int64, locationCode string) (*model.MdmLocation, error)
	FindByID(id int64) (*model.MdmLocation, error)
	List(warehouseID int64) ([]model.MdmLocation, error)
}

type materialRepo struct {
	db *gorm.DB
}

func NewMaterialRepository(db *gorm.DB) MaterialRepository {
	return &materialRepo{db: db}
}

func (r *materialRepo) FindByCode(code string) (*model.MdmMaterial, error) {
	var m model.MdmMaterial
	if err := r.db.Where("material_code = ?", code).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *materialRepo) FindByID(id int64) (*model.MdmMaterial, error) {
	var m model.MdmMaterial
	if err := r.db.Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *materialRepo) Create(material *model.MdmMaterial) error {
	return r.db.Create(material).Error
}

func (r *materialRepo) Update(material *model.MdmMaterial) error {
	return r.db.Save(material).Error
}

func (r *materialRepo) List(code, name string, page, pageSize int) ([]model.MdmMaterial, int64, error) {
	var materials []model.MdmMaterial
	var total int64
	query := r.db.Model(&model.MdmMaterial{})
	if code != "" {
		query = query.Where("material_code LIKE ?", "%"+code+"%")
	}
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&materials).Error; err != nil {
		return nil, 0, err
	}
	return materials, total, nil
}

type supplierRepo struct {
	db *gorm.DB
}

func NewSupplierRepository(db *gorm.DB) SupplierRepository {
	return &supplierRepo{db: db}
}

func (r *supplierRepo) FindByCode(code string) (*model.MdmSupplier, error) {
	var s model.MdmSupplier
	if err := r.db.Where("supplier_code = ?", code).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *supplierRepo) FindByID(id int64) (*model.MdmSupplier, error) {
	var s model.MdmSupplier
	if err := r.db.Where("id = ?", id).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *supplierRepo) Create(supplier *model.MdmSupplier) error {
	return r.db.Create(supplier).Error
}

func (r *supplierRepo) Update(supplier *model.MdmSupplier) error {
	return r.db.Save(supplier).Error
}

func (r *supplierRepo) List(code, name string, page, pageSize int) ([]model.MdmSupplier, int64, error) {
	var suppliers []model.MdmSupplier
	var total int64
	query := r.db.Model(&model.MdmSupplier{})
	if code != "" {
		query = query.Where("supplier_code LIKE ?", "%"+code+"%")
	}
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&suppliers).Error; err != nil {
		return nil, 0, err
	}
	return suppliers, total, nil
}

type warehouseRepo struct {
	db *gorm.DB
}

func NewWarehouseRepository(db *gorm.DB) WarehouseRepository {
	return &warehouseRepo{db: db}
}

func (r *warehouseRepo) FindByCode(code string) (*model.MdmWarehouse, error) {
	var w model.MdmWarehouse
	if err := r.db.Where("code = ?", code).First(&w).Error; err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *warehouseRepo) FindByID(id int64) (*model.MdmWarehouse, error) {
	var w model.MdmWarehouse
	if err := r.db.Where("id = ?", id).First(&w).Error; err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *warehouseRepo) List() ([]model.MdmWarehouse, error) {
	var warehouses []model.MdmWarehouse
	if err := r.db.Find(&warehouses).Error; err != nil {
		return nil, err
	}
	return warehouses, nil
}

type locationRepo struct {
	db *gorm.DB
}

func NewLocationRepository(db *gorm.DB) LocationRepository {
	return &locationRepo{db: db}
}

func (r *locationRepo) FindByCode(warehouseID int64, locationCode string) (*model.MdmLocation, error) {
	var l model.MdmLocation
	if err := r.db.Where("warehouse_id = ? AND location_code = ?", warehouseID, locationCode).First(&l).Error; err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *locationRepo) FindByID(id int64) (*model.MdmLocation, error) {
	var l model.MdmLocation
	if err := r.db.Where("id = ?", id).First(&l).Error; err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *locationRepo) List(warehouseID int64) ([]model.MdmLocation, error) {
	var locations []model.MdmLocation
	query := r.db.Model(&model.MdmLocation{})
	if warehouseID > 0 {
		query = query.Where("warehouse_id = ?", warehouseID)
	}
	if err := query.Find(&locations).Error; err != nil {
		return nil, err
	}
	return locations, nil
}
