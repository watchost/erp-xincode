// Copyright 2026 zhouhouping. All Rights Reserved.

package repository

import (
	"errors"

	"github.com/shopspring/decimal"
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
	// Exists* 用于主数据删除前的引用守卫：判断是否仍有在库余额（qty > 0）。
	ExistsMaterialOnHand(materialID int64) (bool, error)
	ExistsWarehouseInUse(warehouseID int64) (bool, error)
	ExistsLocationInUse(locationID int64) (bool, error)
	SumAvailableQty(materialID int64) (decimal.Decimal, error)
	ListByWarehouse(warehouseID int64, locationID *int64) ([]model.InvInventory, error)
	GetMaterialAvgCost(materialID int64) (decimal.Decimal, bool, error)
}

type StockLedgerRepository interface {
	Append(tx *gorm.DB, ledger *model.InvStockLedger) error
	List(materialID int64, bizType int, page, pageSize int) ([]model.InvStockLedger, int64, error)
}

type StockCheckRepository interface {
	Create(tx *gorm.DB, check *model.InvStockCheck) error
	Update(tx *gorm.DB, check *model.InvStockCheck) error
	FindByID(id int64) (*model.InvStockCheck, error)
	FindByNo(checkNo string) (*model.InvStockCheck, error)
	FindByIDForUpdate(tx *gorm.DB, id int64) (*model.InvStockCheck, error)
	List(warehouseID int64, status int16, checkNo string, page, pageSize int) ([]*model.InvStockCheck, int64, error)
	IsWarehouseFrozen(warehouseID int64) (bool, error)
	CreateItems(tx *gorm.DB, items []*model.InvStockCheckItem) error
	DeleteItemsByCheckID(tx *gorm.DB, checkID int64) error
	FindItemsByCheckID(checkID int64) ([]*model.InvStockCheckItem, error)
}

type TransferRepository interface {
	Create(tx *gorm.DB, transfer *model.InvTransfer) error
	Update(tx *gorm.DB, transfer *model.InvTransfer) error
	FindByID(id int64) (*model.InvTransfer, error)
	FindByNo(transferNo string) (*model.InvTransfer, error)
	FindByIDForUpdate(tx *gorm.DB, id int64) (*model.InvTransfer, error)
	List(fromWarehouseID, toWarehouseID int64, status int16, transferNo string, page, pageSize int) ([]*model.InvTransfer, int64, error)
	CreateItems(tx *gorm.DB, items []*model.InvTransferItem) error
	UpdateItem(tx *gorm.DB, item *model.InvTransferItem) error
	FindItemsByTransferID(transferID int64) ([]*model.InvTransferItem, error)
}

type BatchRepository interface {
	Create(tx *gorm.DB, batch *model.InvBatch) error
	Update(tx *gorm.DB, batch *model.InvBatch) error
	FindByID(id int64) (*model.InvBatch, error)
	FindByNo(batchNo string, materialID int64) (*model.InvBatch, error)
	List(materialID int64, batchNo string, page, pageSize int) ([]*model.InvBatch, int64, error)
	ListAvailableFEFO(materialID int64) ([]*model.InvBatch, error)
	DeductQty(tx *gorm.DB, batchID int64, qty decimal.Decimal) error
}

type SerialRepository interface {
	CreateBatch(tx *gorm.DB, serials []*model.InvSerial) error
	Update(tx *gorm.DB, serial *model.InvSerial) error
	FindByID(id int64) (*model.InvSerial, error)
	FindByNo(serialNo string, materialID int64) (*model.InvSerial, error)
	List(materialID int64, serialNo string, status int16, page, pageSize int) ([]*model.InvSerial, int64, error)
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

func (r *inventoryRepo) existsWhere(where string, arg int64) (bool, error) {
	var count int64
	err := r.db.Model(&model.InvInventory{}).
		Where(where, arg).
		Where("qty > 0").
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *inventoryRepo) ExistsMaterialOnHand(materialID int64) (bool, error) {
	return r.existsWhere("material_id = ?", materialID)
}

func (r *inventoryRepo) ExistsWarehouseInUse(warehouseID int64) (bool, error) {
	return r.existsWhere("warehouse_id = ?", warehouseID)
}

func (r *inventoryRepo) ExistsLocationInUse(locationID int64) (bool, error) {
	return r.existsWhere("location_id = ?", locationID)
}

// SumAvailableQty 跨仓合计某物料的可用库存（工单齐套检查用）。无库存行时返回 0。
func (r *inventoryRepo) SumAvailableQty(materialID int64) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := r.db.Model(&model.InvInventory{}).
		Select("COALESCE(SUM(available_qty), 0)").
		Where("material_id = ?", materialID).
		Scan(&total).Error
	if err != nil {
		return decimal.Zero, err
	}
	return total, nil
}

func (r *inventoryRepo) ListByWarehouse(warehouseID int64, locationID *int64) ([]model.InvInventory, error) {
	var list []model.InvInventory
	q := r.db.Where("warehouse_id = ?", warehouseID)
	if locationID != nil && *locationID > 0 {
		q = q.Where("location_id = ?", *locationID)
	}
	if err := q.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *inventoryRepo) GetMaterialAvgCost(materialID int64) (decimal.Decimal, bool, error) {
	var result struct {
		TotalQty  decimal.Decimal
		TotalCost decimal.Decimal
	}
	err := r.db.Model(&model.InvInventory{}).
		Select("COALESCE(SUM(qty), 0) as total_qty, COALESCE(SUM(qty * avg_cost), 0) as total_cost").
		Where("material_id = ? AND qty > 0", materialID).
		Scan(&result).Error
	if err != nil {
		return decimal.Zero, false, err
	}
	if result.TotalQty.IsPositive() {
		return result.TotalCost.Div(result.TotalQty), true, nil
	}
	var latest model.InvInventory
	if err := r.db.Where("material_id = ? AND avg_cost > 0", materialID).Order("updated_at DESC").First(&latest).Error; err == nil {
		return latest.AvgCost, true, nil
	}
	return decimal.Zero, false, nil
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

type stockCheckRepo struct {
	db *gorm.DB
}

func NewStockCheckRepository(db *gorm.DB) StockCheckRepository {
	return &stockCheckRepo{db: db}
}

func (r *stockCheckRepo) Create(tx *gorm.DB, check *model.InvStockCheck) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Create(check).Error
}

func (r *stockCheckRepo) Update(tx *gorm.DB, check *model.InvStockCheck) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Save(check).Error
}

func (r *stockCheckRepo) FindByID(id int64) (*model.InvStockCheck, error) {
	var check model.InvStockCheck
	if err := r.db.Preload("Items").Where("id = ?", id).First(&check).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &check, nil
}

func (r *stockCheckRepo) FindByNo(checkNo string) (*model.InvStockCheck, error) {
	var check model.InvStockCheck
	if err := r.db.Preload("Items").Where("check_no = ?", checkNo).First(&check).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &check, nil
}

func (r *stockCheckRepo) FindByIDForUpdate(tx *gorm.DB, id int64) (*model.InvStockCheck, error) {
	var check model.InvStockCheck
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Items").Where("id = ?", id).First(&check).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &check, nil
}

func (r *stockCheckRepo) List(warehouseID int64, status int16, checkNo string, page, pageSize int) ([]*model.InvStockCheck, int64, error) {
	var list []*model.InvStockCheck
	var total int64
	q := r.db.Model(&model.InvStockCheck{})
	if warehouseID > 0 {
		q = q.Where("warehouse_id = ?", warehouseID)
	}
	if status > 0 {
		q = q.Where("status = ?", status)
	}
	if checkNo != "" {
		q = q.Where("check_no LIKE ?", "%"+checkNo+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Preload("Items").Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *stockCheckRepo) IsWarehouseFrozen(warehouseID int64) (bool, error) {
	var count int64
	err := r.db.Model(&model.InvStockCheck{}).
		Where("warehouse_id = ? AND frozen = TRUE AND status IN (?, ?)", warehouseID, model.StockCheckStatusFrozen, model.StockCheckStatusCounted).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *stockCheckRepo) CreateItems(tx *gorm.DB, items []*model.InvStockCheckItem) error {
	if len(items) == 0 {
		return nil
	}
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Create(&items).Error
}

func (r *stockCheckRepo) DeleteItemsByCheckID(tx *gorm.DB, checkID int64) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Where("check_id = ?", checkID).Delete(&model.InvStockCheckItem{}).Error
}

func (r *stockCheckRepo) FindItemsByCheckID(checkID int64) ([]*model.InvStockCheckItem, error) {
	var items []*model.InvStockCheckItem
	if err := r.db.Where("check_id = ?", checkID).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

type transferRepo struct {
	db *gorm.DB
}

func NewTransferRepository(db *gorm.DB) TransferRepository {
	return &transferRepo{db: db}
}

func (r *transferRepo) Create(tx *gorm.DB, transfer *model.InvTransfer) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Create(transfer).Error
}

func (r *transferRepo) Update(tx *gorm.DB, transfer *model.InvTransfer) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Save(transfer).Error
}

func (r *transferRepo) FindByID(id int64) (*model.InvTransfer, error) {
	var transfer model.InvTransfer
	if err := r.db.Preload("Items").Where("id = ?", id).First(&transfer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &transfer, nil
}

func (r *transferRepo) FindByNo(transferNo string) (*model.InvTransfer, error) {
	var transfer model.InvTransfer
	if err := r.db.Preload("Items").Where("transfer_no = ?", transferNo).First(&transfer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &transfer, nil
}

func (r *transferRepo) FindByIDForUpdate(tx *gorm.DB, id int64) (*model.InvTransfer, error) {
	var transfer model.InvTransfer
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Items").Where("id = ?", id).First(&transfer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &transfer, nil
}

func (r *transferRepo) List(fromWarehouseID, toWarehouseID int64, status int16, transferNo string, page, pageSize int) ([]*model.InvTransfer, int64, error) {
	var list []*model.InvTransfer
	var total int64
	q := r.db.Model(&model.InvTransfer{})
	if fromWarehouseID > 0 {
		q = q.Where("from_warehouse_id = ?", fromWarehouseID)
	}
	if toWarehouseID > 0 {
		q = q.Where("to_warehouse_id = ?", toWarehouseID)
	}
	if status > 0 {
		q = q.Where("status = ?", status)
	}
	if transferNo != "" {
		q = q.Where("transfer_no LIKE ?", "%"+transferNo+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Preload("Items").Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *transferRepo) CreateItems(tx *gorm.DB, items []*model.InvTransferItem) error {
	if len(items) == 0 {
		return nil
	}
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Create(&items).Error
}

func (r *transferRepo) UpdateItem(tx *gorm.DB, item *model.InvTransferItem) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Save(item).Error
}

func (r *transferRepo) FindItemsByTransferID(transferID int64) ([]*model.InvTransferItem, error) {
	var items []*model.InvTransferItem
	if err := r.db.Where("transfer_id = ?", transferID).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

type batchRepo struct {
	db *gorm.DB
}

func NewBatchRepository(db *gorm.DB) BatchRepository {
	return &batchRepo{db: db}
}

func (r *batchRepo) Create(tx *gorm.DB, batch *model.InvBatch) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Create(batch).Error
}

func (r *batchRepo) Update(tx *gorm.DB, batch *model.InvBatch) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Save(batch).Error
}

func (r *batchRepo) FindByID(id int64) (*model.InvBatch, error) {
	var batch model.InvBatch
	if err := r.db.Where("id = ?", id).First(&batch).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &batch, nil
}

func (r *batchRepo) FindByNo(batchNo string, materialID int64) (*model.InvBatch, error) {
	var batch model.InvBatch
	if err := r.db.Where("batch_no = ? AND material_id = ?", batchNo, materialID).First(&batch).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &batch, nil
}

func (r *batchRepo) List(materialID int64, batchNo string, page, pageSize int) ([]*model.InvBatch, int64, error) {
	var list []*model.InvBatch
	var total int64
	q := r.db.Model(&model.InvBatch{})
	if materialID > 0 {
		q = q.Where("material_id = ?", materialID)
	}
	if batchNo != "" {
		q = q.Where("batch_no LIKE ?", "%"+batchNo+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *batchRepo) ListAvailableFEFO(materialID int64) ([]*model.InvBatch, error) {
	var list []*model.InvBatch
	err := r.db.Where("material_id = ? AND qty_remaining > 0", materialID).
		Order("expired_at ASC NULLS LAST, created_at ASC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (r *batchRepo) DeductQty(tx *gorm.DB, batchID int64, qty decimal.Decimal) error {
	res := tx.Model(&model.InvBatch{}).
		Where("id = ? AND qty_remaining >= ?", batchID, qty).
		Update("qty_remaining", gorm.Expr("qty_remaining - ?", qty))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("批次余量不足")
	}
	return nil
}

type serialRepo struct {
	db *gorm.DB
}

func NewSerialRepository(db *gorm.DB) SerialRepository {
	return &serialRepo{db: db}
}

func (r *serialRepo) CreateBatch(tx *gorm.DB, serials []*model.InvSerial) error {
	if len(serials) == 0 {
		return nil
	}
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Create(&serials).Error
}

func (r *serialRepo) Update(tx *gorm.DB, serial *model.InvSerial) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Save(serial).Error
}

func (r *serialRepo) FindByID(id int64) (*model.InvSerial, error) {
	var serial model.InvSerial
	if err := r.db.Where("id = ?", id).First(&serial).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &serial, nil
}

func (r *serialRepo) FindByNo(serialNo string, materialID int64) (*model.InvSerial, error) {
	var serial model.InvSerial
	if err := r.db.Where("serial_no = ? AND material_id = ?", serialNo, materialID).First(&serial).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &serial, nil
}

func (r *serialRepo) List(materialID int64, serialNo string, status int16, page, pageSize int) ([]*model.InvSerial, int64, error) {
	var list []*model.InvSerial
	var total int64
	q := r.db.Model(&model.InvSerial{})
	if materialID > 0 {
		q = q.Where("material_id = ?", materialID)
	}
	if serialNo != "" {
		q = q.Where("serial_no LIKE ?", "%"+serialNo+"%")
	}
	if status > 0 {
		q = q.Where("status = ?", status)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}


