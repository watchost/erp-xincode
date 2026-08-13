// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"time"

	"gorm.io/gorm"

	mdmRepo "erp-system/internal/mdm/repository"
	"erp-system/internal/pkg/bizno"
	"erp-system/internal/pkg/db"
	"erp-system/internal/pkg/errors"
	"erp-system/internal/pkg/idemp"
	"erp-system/internal/warehouse/dto"
	warehouseModel "erp-system/internal/warehouse/model"
	"erp-system/internal/warehouse/repository"
)

const (
	BizTypePurchaseInbound    = 1
	BizTypeProductionOutbound = 2
	BizTypeProductionInbound  = 3
	BizTypeInventoryCheck     = 4
	BizTypeLocationTransfer   = 5
	BizTypeSalesOutbound      = 6
)

// IdempotencyGuard 由 *idemp.Guard 满足；抽成接口便于单测。
type IdempotencyGuard interface {
	Acquire(ctx context.Context, scope, key string) error
}

type WarehouseService struct {
	txManager     *db.TxManager
	invRepo       repository.InventoryRepository
	ledgerRepo    repository.StockLedgerRepository
	materialRepo  mdmRepo.MaterialRepository
	warehouseRepo mdmRepo.WarehouseRepository
	locationRepo  mdmRepo.LocationRepository
	numbers       *bizno.Generator
	idemp         IdempotencyGuard
}

func NewWarehouseService(
	txManager *db.TxManager,
	invRepo repository.InventoryRepository,
	ledgerRepo repository.StockLedgerRepository,
	materialRepo mdmRepo.MaterialRepository,
	warehouseRepo mdmRepo.WarehouseRepository,
	locationRepo mdmRepo.LocationRepository,
	numbers *bizno.Generator,
	idempGuard IdempotencyGuard,
) *WarehouseService {
	return &WarehouseService{
		txManager:     txManager,
		invRepo:       invRepo,
		ledgerRepo:    ledgerRepo,
		materialRepo:  materialRepo,
		warehouseRepo: warehouseRepo,
		locationRepo:  locationRepo,
		numbers:       numbers,
		idemp:         idempGuard,
	}
}

func (s *WarehouseService) generateNo(ctx context.Context, prefix string) string {
	if s.numbers == nil {
		// 退化方案：不应到达这里，main 必须注入 generator
		return prefix + time.Now().Format("20060102150405")
	}
	return s.numbers.Next(ctx, prefix)
}

func (s *WarehouseService) acquireIdemp(ctx context.Context, scope, key string) error {
	if s.idemp == nil || key == "" {
		return nil
	}
	if err := s.idemp.Acquire(ctx, scope, key); err != nil {
		if err == idemp.ErrDuplicate {
			return errors.New(10200, 429, "请求正在处理或已处理，请勿重复提交")
		}
		return err
	}
	return nil
}

func (s *WarehouseService) Inbound(ctx context.Context, req dto.InboundScanReq) (*dto.InboundScanRes, error) {
	if err := s.acquireIdemp(ctx, "warehouse-inbound", req.IdempotencyKey); err != nil {
		return nil, err
	}

	material, err := s.materialRepo.FindByCode(req.MaterialCode)
	if err != nil {
		return nil, errors.New(30002, 404, "物料不存在")
	}
	warehouse, err := s.warehouseRepo.FindByCode(req.WarehouseCode)
	if err != nil {
		return nil, errors.New(30002, 404, "仓库不存在")
	}
	location, err := s.locationRepo.FindByCode(warehouse.ID, req.LocationCode)
	if err != nil {
		return nil, errors.New(30002, 404, "库位不存在")
	}

	inboundNo := s.generateNo(ctx, "IB")

	var res dto.MoveResult
	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		r, err := s.ApplyInboundTx(ctx, tx, dto.MoveInput{
			MaterialID:  material.ID,
			WarehouseID: warehouse.ID,
			LocationID:  location.ID,
			Qty:         req.Qty,
			UnitCost:    req.UnitCost,
			BizType:     BizTypePurchaseInbound,
			BizNo:       inboundNo,
		})
		if err != nil {
			return err
		}
		res = r
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &dto.InboundScanRes{
		InboundNo: inboundNo,
		Matched:   true,
		AfterQty:  res.AfterQty,
		AvgCost:   res.AvgCost,
	}, nil
}

func (s *WarehouseService) Outbound(ctx context.Context, req dto.OutboundScanReq) (*dto.InboundScanRes, error) {
	if err := s.acquireIdemp(ctx, "warehouse-outbound", req.IdempotencyKey); err != nil {
		return nil, err
	}

	material, err := s.materialRepo.FindByCode(req.MaterialCode)
	if err != nil {
		return nil, errors.New(30002, 404, "物料不存在")
	}
	warehouse, err := s.warehouseRepo.FindByCode(req.WarehouseCode)
	if err != nil {
		return nil, errors.New(30002, 404, "仓库不存在")
	}
	location, err := s.locationRepo.FindByCode(warehouse.ID, req.LocationCode)
	if err != nil {
		return nil, errors.New(30002, 404, "库位不存在")
	}

	outboundNo := req.OutboundNo
	if outboundNo == "" {
		outboundNo = s.generateNo(ctx, "OB")
	}

	var res dto.MoveResult
	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		r, err := s.ApplyOutboundTx(ctx, tx, dto.MoveInput{
			MaterialID:  material.ID,
			WarehouseID: warehouse.ID,
			LocationID:  location.ID,
			Qty:         req.Qty,
			BizType:     BizTypeProductionOutbound,
			BizNo:       outboundNo,
		})
		if err != nil {
			return err
		}
		res = r
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &dto.InboundScanRes{
		InboundNo: outboundNo,
		Matched:   true,
		AfterQty:  res.AfterQty,
		AvgCost:   res.AvgCost,
	}, nil
}

// ApplyInboundTx 在调用方提供的事务内执行入库。
// 供 PurchaseService 等需要把库存写入与单据更新放在同一事务里的场景使用。
// 移动平均成本：new_avg = (old_avg*old_qty + unit_cost*in_qty) / (old_qty + in_qty)。
// 当 unitCost 为 0（来源未知单价）时保持原 avg_cost，不再把成本冲为 0。
func (s *WarehouseService) ApplyInboundTx(ctx context.Context, tx *gorm.DB, in dto.MoveInput) (dto.MoveResult, error) {
	inv, err := s.invRepo.FindForUpdate(tx, in.MaterialID, in.WarehouseID, in.LocationID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return dto.MoveResult{}, err
	}

	if inv == nil {
		inv = &warehouseModel.InvInventory{
			MaterialID:   in.MaterialID,
			WarehouseID:  in.WarehouseID,
			LocationID:   in.LocationID,
			Qty:          in.Qty,
			AvailableQty: in.Qty,
			AvgCost:      in.UnitCost,
			UpdatedAt:    time.Now(),
		}
	} else {
		oldQty, oldAvg := inv.Qty, inv.AvgCost
		if in.UnitCost > 0 {
			inv.AvgCost = (oldAvg*oldQty + in.UnitCost*in.Qty) / (oldQty + in.Qty)
		}
		inv.Qty = oldQty + in.Qty
		inv.AvailableQty = inv.AvailableQty + in.Qty
		inv.UpdatedAt = time.Now()
	}

	if err := s.invRepo.Upsert(tx, inv); err != nil {
		return dto.MoveResult{}, err
	}

	costAmount := 0.0
	if in.UnitCost > 0 {
		costAmount = in.UnitCost * in.Qty
	}
	if err := s.ledgerRepo.Append(tx, &warehouseModel.InvStockLedger{
		MaterialID:  in.MaterialID,
		WarehouseID: in.WarehouseID,
		BizType:     in.BizType,
		BizNo:       in.BizNo,
		ChangeQty:   in.Qty,
		AfterQty:    inv.Qty,
		CostAmount:  costAmount,
	}); err != nil {
		return dto.MoveResult{}, err
	}

	return dto.MoveResult{AfterQty: inv.Qty, AvgCost: inv.AvgCost}, nil
}

// ApplyOutboundTx 在调用方事务内执行出库并扣减库存。
// 成本按当前移动加权平均单价结转：cost_amount = avg_cost * qty。
func (s *WarehouseService) ApplyOutboundTx(ctx context.Context, tx *gorm.DB, in dto.MoveInput) (dto.MoveResult, error) {
	inv, err := s.invRepo.FindForUpdate(tx, in.MaterialID, in.WarehouseID, in.LocationID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return dto.MoveResult{}, errors.New(30001, 409, "库存不足")
		}
		return dto.MoveResult{}, err
	}
	if inv.AvailableQty < in.Qty || inv.Qty < in.Qty {
		return dto.MoveResult{}, errors.New(30001, 409, "库存不足")
	}

	oldAvg := inv.AvgCost
	inv.Qty -= in.Qty
	inv.AvailableQty -= in.Qty
	inv.UpdatedAt = time.Now()

	if err := s.invRepo.Upsert(tx, inv); err != nil {
		return dto.MoveResult{}, err
	}

	if err := s.ledgerRepo.Append(tx, &warehouseModel.InvStockLedger{
		MaterialID:  in.MaterialID,
		WarehouseID: in.WarehouseID,
		BizType:     in.BizType,
		BizNo:       in.BizNo,
		ChangeQty:   -in.Qty,
		AfterQty:    inv.Qty,
		CostAmount:  oldAvg * in.Qty,
	}); err != nil {
		return dto.MoveResult{}, err
	}

	return dto.MoveResult{AfterQty: inv.Qty, AvgCost: inv.AvgCost}, nil
}

func (s *WarehouseService) ListInventory(ctx context.Context, req dto.InventoryQuery) ([]dto.InventoryVO, int64, error) {
	list, total, err := s.invRepo.List(req.WarehouseID, req.MaterialCode, req.MaterialName, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}

	vos := make([]dto.InventoryVO, 0, len(list))
	for _, inv := range list {
		m, _ := s.materialRepo.FindByID(inv.MaterialID)
		w, _ := s.warehouseRepo.FindByID(inv.WarehouseID)
		l, _ := s.locationRepo.FindByID(inv.LocationID)

		materialCode, materialName := "", ""
		if m != nil {
			materialCode = m.MaterialCode
			materialName = m.Name
		}
		warehouseName := ""
		if w != nil {
			warehouseName = w.Name
		}
		locationCode := ""
		if l != nil {
			locationCode = l.LocationCode
		}

		vos = append(vos, dto.InventoryVO{
			ID:            inv.ID,
			MaterialID:    inv.MaterialID,
			MaterialCode:  materialCode,
			MaterialName:  materialName,
			WarehouseID:   inv.WarehouseID,
			WarehouseName: warehouseName,
			LocationID:    inv.LocationID,
			LocationCode:  locationCode,
			Qty:           inv.Qty,
			AvailableQty:  inv.AvailableQty,
			AvgCost:       inv.AvgCost,
			TotalValue:    inv.Qty * inv.AvgCost,
		})
	}
	return vos, total, nil
}

func (s *WarehouseService) GetStockAlerts(ctx context.Context) ([]dto.StockAlertVO, error) {
	var inventories []warehouseModel.InvInventory
	if err := s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		return tx.Where("available_qty < 10").Find(&inventories).Error
	}); err != nil {
		return nil, err
	}

	vos := make([]dto.StockAlertVO, 0, len(inventories))
	for _, inv := range inventories {
		m, _ := s.materialRepo.FindByID(inv.MaterialID)
		w, _ := s.warehouseRepo.FindByID(inv.WarehouseID)
		materialCode, materialName := "", ""
		if m != nil {
			materialCode = m.MaterialCode
			materialName = m.Name
		}
		warehouseName := ""
		if w != nil {
			warehouseName = w.Name
		}
		level := "low"
		if inv.AvailableQty < 5 {
			level = "high"
		}
		vos = append(vos, dto.StockAlertVO{
			ID:            inv.ID,
			MaterialCode:  materialCode,
			MaterialName:  materialName,
			WarehouseName: warehouseName,
			CurrentQty:    inv.AvailableQty,
			MinQty:        10,
			Level:         level,
		})
	}
	return vos, nil
}
