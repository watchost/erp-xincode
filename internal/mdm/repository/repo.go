// Copyright 2026 zhouhouping. All Rights Reserved.

package repository

import (
	"erp-system/internal/mdm/model"
	"gorm.io/gorm"
)

type MaterialRepository interface {
	FindByCode(code string) (*model.MdmMaterial, error)
	FindByID(id int64) (*model.MdmMaterial, error)
	Create(material *model.MdmMaterial) error
	Update(material *model.MdmMaterial) error
	Delete(id int64) error
	List(code, name string, page, pageSize int) ([]model.MdmMaterial, int64, error)
	CountByCategory(categoryID int64) (int64, error)
}

type CategoryRepository interface {
	FindByID(id int64) (*model.MdmMaterialCategory, error)
	FindByCode(code string) (*model.MdmMaterialCategory, error)
	List() ([]model.MdmMaterialCategory, error)
	Create(c *model.MdmMaterialCategory) error
	Update(c *model.MdmMaterialCategory) error
	Delete(id int64) error
	CountChildren(parentID int64) (int64, error)
}

type CustomerRepository interface {
	FindByCode(code string) (*model.MdmCustomer, error)
	FindByID(id int64) (*model.MdmCustomer, error)
	Create(c *model.MdmCustomer) error
	Update(c *model.MdmCustomer) error
	Delete(id int64) error
	List(code, name string, page, pageSize int) ([]model.MdmCustomer, int64, error)
}

type SupplierRepository interface {
	FindByCode(code string) (*model.MdmSupplier, error)
	FindByID(id int64) (*model.MdmSupplier, error)
	Create(supplier *model.MdmSupplier) error
	Update(supplier *model.MdmSupplier) error
	Delete(id int64) error
	List(code, name string, page, pageSize int) ([]model.MdmSupplier, int64, error)
}

type WarehouseRepository interface {
	FindByCode(code string) (*model.MdmWarehouse, error)
	FindByID(id int64) (*model.MdmWarehouse, error)
	List() ([]model.MdmWarehouse, error)
	Create(w *model.MdmWarehouse) error
	Update(w *model.MdmWarehouse) error
	Delete(id int64) error
}

type LocationRepository interface {
	FindByCode(warehouseID int64, locationCode string) (*model.MdmLocation, error)
	FindByID(id int64) (*model.MdmLocation, error)
	List(warehouseID int64) ([]model.MdmLocation, error)
	Create(l *model.MdmLocation) error
	Update(l *model.MdmLocation) error
	Delete(id int64) error
	CountByWarehouse(warehouseID int64) (int64, error)
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

func (r *materialRepo) Delete(id int64) error {
	return r.db.Delete(&model.MdmMaterial{}, id).Error
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

func (r *supplierRepo) Delete(id int64) error {
	return r.db.Delete(&model.MdmSupplier{}, id).Error
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

func (r *warehouseRepo) Create(w *model.MdmWarehouse) error {
	return r.db.Create(w).Error
}

func (r *warehouseRepo) Update(w *model.MdmWarehouse) error {
	return r.db.Save(w).Error
}

func (r *warehouseRepo) Delete(id int64) error {
	return r.db.Delete(&model.MdmWarehouse{}, id).Error
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

func (r *locationRepo) Create(l *model.MdmLocation) error {
	return r.db.Create(l).Error
}

func (r *locationRepo) Update(l *model.MdmLocation) error {
	return r.db.Save(l).Error
}

func (r *locationRepo) Delete(id int64) error {
	return r.db.Delete(&model.MdmLocation{}, id).Error
}

func (r *locationRepo) CountByWarehouse(warehouseID int64) (int64, error) {
	var count int64
	err := r.db.Model(&model.MdmLocation{}).Where("warehouse_id = ?", warehouseID).Count(&count).Error
	return count, err
}

func (r *materialRepo) CountByCategory(categoryID int64) (int64, error) {
	var count int64
	err := r.db.Model(&model.MdmMaterial{}).Where("category_id = ?", categoryID).Count(&count).Error
	return count, err
}

type categoryRepo struct {
	db *gorm.DB
}

func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepo{db: db}
}

func (r *categoryRepo) FindByID(id int64) (*model.MdmMaterialCategory, error) {
	var c model.MdmMaterialCategory
	if err := r.db.Where("id = ?", id).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *categoryRepo) FindByCode(code string) (*model.MdmMaterialCategory, error) {
	var c model.MdmMaterialCategory
	if err := r.db.Where("code = ?", code).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *categoryRepo) List() ([]model.MdmMaterialCategory, error) {
	var list []model.MdmMaterialCategory
	if err := r.db.Order("code").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *categoryRepo) Create(c *model.MdmMaterialCategory) error {
	return r.db.Create(c).Error
}

func (r *categoryRepo) Update(c *model.MdmMaterialCategory) error {
	return r.db.Save(c).Error
}

func (r *categoryRepo) Delete(id int64) error {
	return r.db.Delete(&model.MdmMaterialCategory{}, id).Error
}

func (r *categoryRepo) CountChildren(parentID int64) (int64, error) {
	var count int64
	err := r.db.Model(&model.MdmMaterialCategory{}).Where("parent_id = ?", parentID).Count(&count).Error
	return count, err
}

type customerRepo struct {
	db *gorm.DB
}

func NewCustomerRepository(db *gorm.DB) CustomerRepository {
	return &customerRepo{db: db}
}

func (r *customerRepo) FindByCode(code string) (*model.MdmCustomer, error) {
	var c model.MdmCustomer
	if err := r.db.Where("customer_code = ?", code).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *customerRepo) FindByID(id int64) (*model.MdmCustomer, error) {
	var c model.MdmCustomer
	if err := r.db.Where("id = ?", id).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *customerRepo) Create(c *model.MdmCustomer) error {
	return r.db.Create(c).Error
}

func (r *customerRepo) Update(c *model.MdmCustomer) error {
	return r.db.Save(c).Error
}

func (r *customerRepo) Delete(id int64) error {
	return r.db.Delete(&model.MdmCustomer{}, id).Error
}

func (r *customerRepo) List(code, name string, page, pageSize int) ([]model.MdmCustomer, int64, error) {
	var customers []model.MdmCustomer
	var total int64
	query := r.db.Model(&model.MdmCustomer{})
	if code != "" {
		query = query.Where("customer_code LIKE ?", "%"+code+"%")
	}
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&customers).Error; err != nil {
		return nil, 0, err
	}
	return customers, total, nil
}
