// Copyright 2026 zhouhouping. All Rights Reserved.

package repository

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"erp-system/internal/warehouse/model"
)

type InventoryRepository interface {
	// FindForUpdate 在事务内以 SELECT ... FOR UPDATE 锁定库存行，防止读-改-写竞态。
	// 若记录不存在返回 (nil, gorm.ErrRecordNotFound)。
	FindForUpdate(tx *gorm.DB, materialID, warehouseID, locationID int64) (*model.InvInventory, error)
	// Upsert 插入或更新库存行（依赖 (material_id, warehouse_id, location_id) 唯一约束）。
	Upsert(tx *gorm.DB, inv *model.InvInventory) error
	List(warehouseID int64, materialCode, materialName string, page, pageSize int) ([]model.InvInventory, int64, error)
}

type StockLedgerRepository interface {
	Append(tx *gorm.DB, ledger *model.InvStockLedger) error
	List(materialID int64, bizType int, page, pageSize int) ([]model.InvStockLedger, int64, error)
}

type inventoryRepo struct {
	db *gorm.DB
}

func NewInventoryRepository(db *gorm.DB) InventoryRepository {
	return &inventoryRepo{db: db}
}

func (r *inventoryRepo) FindForUpdate(tx *gorm.DB, materialID, warehouseID, locationID int64) (*model.InvInventory, error) {
	var inv model.InvInventory
	q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("material_id = ? AND warehouse_id = ?", materialID, warehouseID)
	if locationID > 0 {
		q = q.Where("location_id = ?", locationID)
	} else {
		q = q.Where("location_id IS NULL")
	}
	if err := q.First(&inv).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &inv, nil
}

func (r *inventoryRepo) Upsert(tx *gorm.DB, inv *model.InvInventory) error {
	// 依赖唯一约束 (material_id, warehouse_id, location_id) 做原子 upsert。
	// 注意：ON CONFLICT 只更新有变化的字段，避免覆盖 FOR UPDATE 锁之外的并发写入。
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "material_id"},
			{Name: "warehouse_id"},
			{Name: "location_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"qty", "available_qty", "avg_cost", "updated_at",
		}),
	}).Create(inv).Error
}

func (r *inventoryRepo) List(warehouseID int64, materialCode, materialName string, page, pageSize int) ([]model.InvInventory, int64, error) {
	var list []model.InvInventory
	var total int64
	query := r.db.Model(&model.InvInventory{})
	if warehouseID > 0 {
		query = query.Where("warehouse_id = ?", warehouseID)
	}
	if materialCode != "" {
		query = query.Joins("JOIN mdm_material ON mdm_material.id = inv_inventory.material_id").
			Where("mdm_material.material_code LIKE ?", "%"+materialCode+"%")
	}
	if materialName != "" {
		query = query.Joins("JOIN mdm_material ON mdm_material.id = inv_inventory.material_id").
			Where("mdm_material.name LIKE ?", "%"+materialName+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

type stockLedgerRepo struct {
	db *gorm.DB
}

func NewStockLedgerRepository(db *gorm.DB) StockLedgerRepository {
	return &stockLedgerRepo{db: db}
}

func (r *stockLedgerRepo) Append(tx *gorm.DB, ledger *model.InvStockLedger) error {
	return tx.Create(ledger).Error
}

func (r *stockLedgerRepo) List(materialID int64, bizType int, page, pageSize int) ([]model.InvStockLedger, int64, error) {
	var list []model.InvStockLedger
	var total int64
	query := r.db.Model(&model.InvStockLedger{})
	if materialID > 0 {
		query = query.Where("material_id = ?", materialID)
	}
	if bizType > 0 {
		query = query.Where("biz_type = ?", bizType)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
