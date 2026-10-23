// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	stdErrors "errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	financeService "erp-system/internal/finance/service"
	mdmRepo "erp-system/internal/mdm/repository"
	"erp-system/internal/pkg/bizno"
	"erp-system/internal/pkg/db"
	"erp-system/internal/pkg/decimalx"
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
	BizTypeSalesReturn        = 7 // 销售退货入库（P2 §3.1）
)

// IdempotencyGuard 由 *idemp.Guard 满足；抽成接口便于单测。
type IdempotencyGuard interface {
	Acquire(ctx context.Context, scope, key string) error
}

type FinancePoster interface {
	PostTx(ctx context.Context, tx *gorm.DB, in financeService.PostingInput) error
}

type WarehouseService struct {
	txManager       *db.TxManager
	invRepo         repository.InventoryRepository
	ledgerRepo      repository.StockLedgerRepository
	stockCheckRepo  repository.StockCheckRepository
	transferRepo    repository.TransferRepository
	materialRepo    mdmRepo.MaterialRepository
	warehouseRepo   mdmRepo.WarehouseRepository
	locationRepo    mdmRepo.LocationRepository
	numbers         *bizno.Generator
	idemp           IdempotencyGuard
	finance         FinancePoster
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

func (s *WarehouseService) SetStockCheckRepository(r repository.StockCheckRepository) {
	s.stockCheckRepo = r
}

func (s *WarehouseService) SetTransferRepository(r repository.TransferRepository) {
	s.transferRepo = r
}

func (s *WarehouseService) SetFinance(f FinancePoster) {
	s.finance = f
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
	if in.BizType != BizTypeInventoryCheck && s.stockCheckRepo != nil {
		frozen, err := s.stockCheckRepo.IsWarehouseFrozen(in.WarehouseID)
		if err != nil {
			return dto.MoveResult{}, err
		}
		if frozen {
			return dto.MoveResult{}, errors.New(40008, 409, "目标仓库处于盘点冻结状态，禁止出入库")
		}
	}

	inv, err := s.invRepo.FindForUpdate(tx, in.MaterialID, in.WarehouseID, in.LocationID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return dto.MoveResult{}, err
	}

	// 数量按 NUMERIC(18,3) 落库标度归整；均价按 NUMERIC(18,4)。
	inQty := decimalx.Qty(in.Qty)
	if inv == nil {
		inv = &warehouseModel.InvInventory{
			MaterialID:   in.MaterialID,
			WarehouseID:  in.WarehouseID,
			LocationID:   in.LocationID,
			Qty:          inQty,
			AvailableQty: inQty,
			AvgCost:      decimalx.Cost(in.UnitCost),
			UpdatedAt:    time.Now(),
		}
	} else {
		oldQty, oldAvg := inv.Qty, inv.AvgCost
		if in.UnitCost.IsPositive() {
			// new_avg = (old_avg*old_qty + unit_cost*in_qty) / (old_qty + in_qty)
			// 除不尽时 DivRound 到 8 位保护精度，再按成本列 4 位归整。
			num := oldAvg.Mul(oldQty).Add(in.UnitCost.Mul(inQty))
			den := oldQty.Add(inQty)
			inv.AvgCost = decimalx.Cost(num.DivRound(den, 8))
		}
		inv.Qty = oldQty.Add(inQty)
		inv.AvailableQty = inv.AvailableQty.Add(inQty)
		inv.UpdatedAt = time.Now()
	}

	if err := s.invRepo.Upsert(tx, inv); err != nil {
		return dto.MoveResult{}, err
	}

	// 单价为 0（来源未知成本）的入库不产生金额，保持原均价，台账金额记 0。
	costAmount := decimal.Zero
	if in.UnitCost.IsPositive() {
		costAmount = decimalx.Money(in.UnitCost.Mul(inQty))
	}
	if err := s.ledgerRepo.Append(tx, &warehouseModel.InvStockLedger{
		MaterialID:  in.MaterialID,
		WarehouseID: in.WarehouseID,
		BizType:     in.BizType,
		BizNo:       in.BizNo,
		ChangeQty:   inQty,
		AfterQty:    inv.Qty,
		CostAmount:  costAmount,
	}); err != nil {
		return dto.MoveResult{}, err
	}

	return dto.MoveResult{AfterQty: inv.Qty, AvgCost: inv.AvgCost, CostAmount: costAmount}, nil
}

// ApplyOutboundTx 在调用方事务内执行出库并扣减库存。
// 成本按当前移动加权平均单价结转：cost_amount = avg_cost * qty。
func (s *WarehouseService) ApplyOutboundTx(ctx context.Context, tx *gorm.DB, in dto.MoveInput) (dto.MoveResult, error) {
	if in.BizType != BizTypeInventoryCheck && s.stockCheckRepo != nil {
		frozen, err := s.stockCheckRepo.IsWarehouseFrozen(in.WarehouseID)
		if err != nil {
			return dto.MoveResult{}, err
		}
		if frozen {
			return dto.MoveResult{}, errors.New(40008, 409, "目标仓库处于盘点冻结状态，禁止出入库")
		}
	}

	inv, err := s.invRepo.FindForUpdate(tx, in.MaterialID, in.WarehouseID, in.LocationID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return dto.MoveResult{}, errors.New(30001, 409, "库存不足")
		}
		return dto.MoveResult{}, err
	}
	outQty := decimalx.Qty(in.Qty)
	if inv.AvailableQty.Cmp(outQty) < 0 || inv.Qty.Cmp(outQty) < 0 {
		return dto.MoveResult{}, errors.New(30001, 409, "库存不足")
	}

	oldAvg := inv.AvgCost
	inv.Qty = inv.Qty.Sub(outQty)
	inv.AvailableQty = inv.AvailableQty.Sub(outQty)
	inv.UpdatedAt = time.Now()

	if err := s.invRepo.Upsert(tx, inv); err != nil {
		return dto.MoveResult{}, err
	}

	costAmount := decimalx.Money(oldAvg.Mul(outQty))
	if err := s.ledgerRepo.Append(tx, &warehouseModel.InvStockLedger{
		MaterialID:  in.MaterialID,
		WarehouseID: in.WarehouseID,
		BizType:     in.BizType,
		BizNo:       in.BizNo,
		ChangeQty:   outQty.Neg(),
		AfterQty:    inv.Qty,
		CostAmount:  costAmount,
	}); err != nil {
		return dto.MoveResult{}, err
	}

	return dto.MoveResult{AfterQty: inv.Qty, AvgCost: inv.AvgCost, CostAmount: costAmount}, nil
}

// MaterialOnHand / WarehouseInUse / LocationInUse 实现 mdm 模块的
// service.InventoryChecker 接口，供主数据删除前做引用守卫。
func (s *WarehouseService) MaterialOnHand(ctx context.Context, materialID int64) (bool, error) {
	return s.invRepo.ExistsMaterialOnHand(materialID)
}

func (s *WarehouseService) WarehouseInUse(ctx context.Context, warehouseID int64) (bool, error) {
	return s.invRepo.ExistsWarehouseInUse(warehouseID)
}

func (s *WarehouseService) LocationInUse(ctx context.Context, locationID int64) (bool, error) {
	return s.invRepo.ExistsLocationInUse(locationID)
}

// AvailableQtyByMaterial 返回某物料跨仓合计的可用库存（生产工单下达前齐套检查用）。
func (s *WarehouseService) AvailableQtyByMaterial(ctx context.Context, materialID int64) (decimal.Decimal, error) {
	return s.invRepo.SumAvailableQty(materialID)
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
			TotalValue:    decimalx.Money(inv.Qty.Mul(inv.AvgCost)),
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
		if inv.AvailableQty.Cmp(decimal.NewFromInt(5)) < 0 {
			level = "high"
		}
		vos = append(vos, dto.StockAlertVO{
			ID:            inv.ID,
			MaterialCode:  materialCode,
			MaterialName:  materialName,
			WarehouseName: warehouseName,
			CurrentQty:    inv.AvailableQty,
			MinQty:        decimal.NewFromInt(10),
			Level:         level,
		})
	}
	return vos, nil
}

// ----------------- P2 §3.2 库存盘点核心业务实现 -----------------

func (s *WarehouseService) CreateStockCheck(ctx context.Context, req dto.CreateStockCheckReq, userID int64) (*warehouseModel.InvStockCheck, error) {
	if _, err := s.warehouseRepo.FindByID(req.WarehouseID); err != nil {
		return nil, errors.New(30002, 404, "仓库不存在")
	}
	if req.LocationID != nil && *req.LocationID > 0 {
		if _, err := s.locationRepo.FindByID(*req.LocationID); err != nil {
			return nil, errors.New(30002, 404, "库位不存在")
		}
	}

	checkNo := s.generateNo(ctx, "SC")
	invList, err := s.invRepo.ListByWarehouse(req.WarehouseID, req.LocationID)
	if err != nil {
		return nil, err
	}

	check := &warehouseModel.InvStockCheck{
		CheckNo:         checkNo,
		WarehouseID:     req.WarehouseID,
		LocationID:      req.LocationID,
		Status:          warehouseModel.StockCheckStatusDraft,
		Frozen:          false,
		TotalDiffAmount: decimal.Zero,
		Remark:          req.Remark,
		CreatedBy:       userID,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	var items []*warehouseModel.InvStockCheckItem
	for _, inv := range invList {
		locID := inv.LocationID
		items = append(items, &warehouseModel.InvStockCheckItem{
			MaterialID: inv.MaterialID,
			LocationID: &locID,
			BookQty:    inv.Qty,
			ActualQty:  inv.Qty,
			DiffQty:    decimal.Zero,
			UnitCost:   inv.AvgCost,
			DiffAmount: decimal.Zero,
		})
	}

	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		if err := s.stockCheckRepo.Create(tx, check); err != nil {
			return err
		}
		for _, item := range items {
			item.CheckID = check.ID
		}
		return s.stockCheckRepo.CreateItems(tx, items)
	})
	if err != nil {
		return nil, err
	}

	check.Items = items
	return check, nil
}

func (s *WarehouseService) FreezeStockCheck(ctx context.Context, checkID int64) error {
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		check, err := s.stockCheckRepo.FindByIDForUpdate(tx, checkID)
		if err != nil {
			if stdErrors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New(40001, 404, "盘点单不存在")
			}
			return err
		}
		if check.Status != warehouseModel.StockCheckStatusDraft {
			return errors.New(40002, 400, "仅草稿状态的盘点单可冻结仓库")
		}
		check.Frozen = true
		check.Status = warehouseModel.StockCheckStatusFrozen
		check.UpdatedAt = time.Now()
		return s.stockCheckRepo.Update(tx, check)
	})
}

func (s *WarehouseService) EntryStockCheck(ctx context.Context, checkID int64, req dto.EntryStockCheckReq) error {
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		check, err := s.stockCheckRepo.FindByIDForUpdate(tx, checkID)
		if err != nil {
			if stdErrors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New(40001, 404, "盘点单不存在")
			}
			return err
		}
		if check.Status != warehouseModel.StockCheckStatusDraft &&
			check.Status != warehouseModel.StockCheckStatusFrozen &&
			check.Status != warehouseModel.StockCheckStatusCounted {
			return errors.New(40003, 400, "当前状态不允许录入实盘数量")
		}

		// 构建实盘录入映射
		type entryKey struct {
			materialID int64
			locationID int64
		}
		entryMap := make(map[entryKey]decimal.Decimal, len(req.Items))
		for _, itemReq := range req.Items {
			locID := int64(0)
			if itemReq.LocationID != nil {
				locID = *itemReq.LocationID
			}
			entryMap[entryKey{materialID: itemReq.MaterialID, locationID: locID}] = decimalx.Qty(itemReq.ActualQty)
		}

		var totalDiff decimal.Decimal
		// 更新现有明细
		for _, item := range check.Items {
			locID := int64(0)
			if item.LocationID != nil {
				locID = *item.LocationID
			}
			key := entryKey{materialID: item.MaterialID, locationID: locID}
			if actual, exists := entryMap[key]; exists {
				item.ActualQty = actual
				item.DiffQty = actual.Sub(item.BookQty)
				item.DiffAmount = decimalx.Money(item.UnitCost.Mul(item.DiffQty))
				totalDiff = totalDiff.Add(item.DiffAmount)
				delete(entryMap, key)
			}
		}

		// 若有账面没有但在实盘中录入的新物料/库位
		for key, actual := range entryMap {
			var locIDPtr *int64
			if key.locationID > 0 {
				loc := key.locationID
				locIDPtr = &loc
			}
			diffQty := actual
			diffAmount := decimal.Zero
			check.Items = append(check.Items, &warehouseModel.InvStockCheckItem{
				CheckID:    check.ID,
				MaterialID: key.materialID,
				LocationID: locIDPtr,
				BookQty:    decimal.Zero,
				ActualQty:  actual,
				DiffQty:    diffQty,
				UnitCost:   decimal.Zero,
				DiffAmount: diffAmount,
			})
			totalDiff = totalDiff.Add(diffAmount)
		}

		if err := s.stockCheckRepo.DeleteItemsByCheckID(tx, check.ID); err != nil {
			return err
		}
		if err := s.stockCheckRepo.CreateItems(tx, check.Items); err != nil {
			return err
		}

		check.TotalDiffAmount = totalDiff
		check.Status = warehouseModel.StockCheckStatusCounted
		check.UpdatedAt = time.Now()
		return s.stockCheckRepo.Update(tx, check)
	})
}

func (s *WarehouseService) ApproveStockCheck(ctx context.Context, checkID int64, approvedBy int64) error {
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		check, err := s.stockCheckRepo.FindByIDForUpdate(tx, checkID)
		if err != nil {
			if stdErrors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New(40001, 404, "盘点单不存在")
			}
			return err
		}
		if check.Status != warehouseModel.StockCheckStatusCounted && check.Status != warehouseModel.StockCheckStatusFrozen {
			return errors.New(40002, 400, "仅盘点中或已录入实盘的盘点单可审批")
		}

		now := time.Now()
		var totalDiffAmount decimal.Decimal
		var gainLines []financeService.PostingLine
		var lossLines []financeService.PostingLine

		for _, item := range check.Items {
			diffQty := item.DiffQty
			if diffQty.IsZero() {
				continue
			}
			locID := int64(0)
			if item.LocationID != nil {
				locID = *item.LocationID
			}

			if diffQty.GreaterThan(decimal.Zero) {
				// 盘盈入库
				inRes, err := s.ApplyInboundTx(ctx, tx, dto.MoveInput{
					MaterialID:  item.MaterialID,
					WarehouseID: check.WarehouseID,
					LocationID:  locID,
					Qty:         diffQty,
					UnitCost:    item.UnitCost,
					BizType:     BizTypeInventoryCheck,
					BizNo:       check.CheckNo,
				})
				if err != nil {
					return err
				}
				diffAmount := decimalx.Money(item.UnitCost.Mul(diffQty))
				item.DiffAmount = diffAmount
				totalDiffAmount = totalDiffAmount.Add(diffAmount)
				_ = inRes

				gainLines = append(gainLines,
					financeService.PostingLine{AccountCode: financeService.AccountFinishedGoods, Debit: diffAmount},
					financeService.PostingLine{AccountCode: financeService.AccountPendingLoss, Credit: diffAmount},
				)
			} else {
				// 盘亏出库
				absDiff := diffQty.Abs()
				outRes, err := s.ApplyOutboundTx(ctx, tx, dto.MoveInput{
					MaterialID:  item.MaterialID,
					WarehouseID: check.WarehouseID,
					LocationID:  locID,
					Qty:         absDiff,
					BizType:     BizTypeInventoryCheck,
					BizNo:       check.CheckNo,
				})
				if err != nil {
					return err
				}
				diffAmount := decimalx.Money(outRes.CostAmount.Neg())
				item.DiffAmount = diffAmount
				totalDiffAmount = totalDiffAmount.Add(diffAmount)

				lossLines = append(lossLines,
					financeService.PostingLine{AccountCode: financeService.AccountPendingLoss, Debit: outRes.CostAmount},
					financeService.PostingLine{AccountCode: financeService.AccountFinishedGoods, Credit: outRes.CostAmount},
				)
			}
		}

		// 财务过账
		if s.finance != nil {
			if len(gainLines) >= 2 {
				if err := s.finance.PostTx(ctx, tx, financeService.PostingInput{
					BizType:     financeService.BizTypeInventoryCheckGain,
					BizNo:       check.CheckNo,
					Description: fmt.Sprintf("盘点单 %s 盘盈入库", check.CheckNo),
					CreatedBy:   approvedBy,
					Lines:       gainLines,
				}); err != nil {
					return err
				}
			}
			if len(lossLines) >= 2 {
				if err := s.finance.PostTx(ctx, tx, financeService.PostingInput{
					BizType:     financeService.BizTypeInventoryCheckLoss,
					BizNo:       check.CheckNo,
					Description: fmt.Sprintf("盘点单 %s 盘亏出库", check.CheckNo),
					CreatedBy:   approvedBy,
					Lines:       lossLines,
				}); err != nil {
					return err
				}
			}
		}

		check.TotalDiffAmount = totalDiffAmount
		check.Frozen = false
		check.Status = warehouseModel.StockCheckStatusCompleted
		check.ApprovedBy = &approvedBy
		check.ApprovedAt = &now
		check.UpdatedAt = now

		return s.stockCheckRepo.Update(tx, check)
	})
}

func (s *WarehouseService) CancelStockCheck(ctx context.Context, checkID int64) error {
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		check, err := s.stockCheckRepo.FindByIDForUpdate(tx, checkID)
		if err != nil {
			if stdErrors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New(40001, 404, "盘点单不存在")
			}
			return err
		}
		if check.Status == warehouseModel.StockCheckStatusCompleted {
			return errors.New(40004, 400, "已完成的盘点单不能取消")
		}
		check.Frozen = false
		check.Status = warehouseModel.StockCheckStatusCancelled
		check.UpdatedAt = time.Now()
		return s.stockCheckRepo.Update(tx, check)
	})
}

func (s *WarehouseService) GetStockCheck(ctx context.Context, id int64) (*warehouseModel.InvStockCheck, error) {
	check, err := s.stockCheckRepo.FindByID(id)
	if err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(40001, 404, "盘点单不存在")
		}
		return nil, err
	}
	return check, nil
}

func (s *WarehouseService) ListStockChecks(ctx context.Context, req dto.StockCheckQuery) ([]*warehouseModel.InvStockCheck, int64, error) {
	return s.stockCheckRepo.List(req.WarehouseID, req.Status, req.CheckNo, req.Page, req.PageSize)
}

// ----------------- P2 §3.3 库存调拨核心业务实现 -----------------

func (s *WarehouseService) CreateTransfer(ctx context.Context, req dto.CreateTransferReq, userID int64) (*warehouseModel.InvTransfer, error) {
	if req.FromWarehouseID == req.ToWarehouseID {
		return nil, errors.New(40009, 400, "调出仓库与调入仓库不能相同")
	}
	if _, err := s.warehouseRepo.FindByID(req.FromWarehouseID); err != nil {
		return nil, errors.New(30002, 404, "调出仓库不存在")
	}
	if _, err := s.warehouseRepo.FindByID(req.ToWarehouseID); err != nil {
		return nil, errors.New(30002, 404, "调入仓库不存在")
	}

	transferNo := s.generateNo(ctx, "TR")
	var transfer *warehouseModel.InvTransfer

	err := s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		var totalAmount decimal.Decimal
		var items []*warehouseModel.InvTransferItem

		for _, itemReq := range req.Items {
			m, err := s.materialRepo.FindByID(itemReq.MaterialID)
			if err != nil || m == nil {
				return errors.New(30002, 404, fmt.Sprintf("物料 ID=%d 不存在", itemReq.MaterialID))
			}
			fromLocID := int64(0)
			if itemReq.FromLocationID != nil {
				fromLocID = *itemReq.FromLocationID
			}
			// 校验调出仓可用库存
			inv, err := s.invRepo.FindForUpdate(tx, itemReq.MaterialID, req.FromWarehouseID, fromLocID)
			if err != nil || inv.AvailableQty.Cmp(itemReq.Qty) < 0 {
				return errors.New(30001, 409, fmt.Sprintf("物料 %s 在调出仓库存不足", m.MaterialCode))
			}

			itemQty := decimalx.Qty(itemReq.Qty)
			unitCost := inv.AvgCost
			itemAmount := decimalx.Money(unitCost.Mul(itemQty))
			totalAmount = totalAmount.Add(itemAmount)

			items = append(items, &warehouseModel.InvTransferItem{
				MaterialID:     itemReq.MaterialID,
				Qty:            itemQty,
				ShippedQty:     decimal.Zero,
				ReceivedQty:    decimal.Zero,
				UnitCost:       unitCost,
				TotalAmount:    itemAmount,
				FromLocationID: itemReq.FromLocationID,
				ToLocationID:   itemReq.ToLocationID,
			})
		}

		transfer = &warehouseModel.InvTransfer{
			TransferNo:      transferNo,
			FromWarehouseID: req.FromWarehouseID,
			ToWarehouseID:   req.ToWarehouseID,
			Status:          warehouseModel.TransferStatusDraft,
			TotalAmount:     totalAmount,
			Remark:          req.Remark,
			CreatedBy:       userID,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}

		if err := s.transferRepo.Create(tx, transfer); err != nil {
			return err
		}
		for _, item := range items {
			item.TransferID = transfer.ID
		}
		if err := s.transferRepo.CreateItems(tx, items); err != nil {
			return err
		}
		transfer.Items = items
		return nil
	})
	if err != nil {
		return nil, err
	}

	return transfer, nil
}

func (s *WarehouseService) ApproveTransfer(ctx context.Context, transferID int64, approvedBy int64) error {
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		transfer, err := s.transferRepo.FindByIDForUpdate(tx, transferID)
		if err != nil {
			if stdErrors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New(40010, 404, "调拨单不存在")
			}
			return err
		}
		if transfer.Status != warehouseModel.TransferStatusDraft {
			return errors.New(40011, 400, "仅草稿状态的调拨单可审批")
		}
		transfer.Status = warehouseModel.TransferStatusApproved
		transfer.UpdatedAt = time.Now()
		return s.transferRepo.Update(tx, transfer)
	})
}

func (s *WarehouseService) ShipTransfer(ctx context.Context, transferID int64) error {
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		transfer, err := s.transferRepo.FindByIDForUpdate(tx, transferID)
		if err != nil {
			if stdErrors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New(40010, 404, "调拨单不存在")
			}
			return err
		}
		if transfer.Status != warehouseModel.TransferStatusApproved {
			return errors.New(40011, 400, "仅已审批的调拨单可发货")
		}

		now := time.Now()
		var totalAmount decimal.Decimal
		for _, item := range transfer.Items {
			fromLocID := int64(0)
			if item.FromLocationID != nil {
				fromLocID = *item.FromLocationID
			}
			outRes, err := s.ApplyOutboundTx(ctx, tx, dto.MoveInput{
				MaterialID:  item.MaterialID,
				WarehouseID: transfer.FromWarehouseID,
				LocationID:  fromLocID,
				Qty:         item.Qty,
				BizType:     BizTypeLocationTransfer,
				BizNo:       transfer.TransferNo,
			})
			if err != nil {
				return err
			}
			item.UnitCost = outRes.AvgCost
			item.ShippedQty = item.Qty
			item.TotalAmount = outRes.CostAmount
			totalAmount = totalAmount.Add(outRes.CostAmount)
			if err := s.transferRepo.UpdateItem(tx, item); err != nil {
				return err
			}
		}

		transfer.TotalAmount = totalAmount
		transfer.Status = warehouseModel.TransferStatusShipped
		transfer.ShippedAt = &now
		transfer.UpdatedAt = now

		return s.transferRepo.Update(tx, transfer)
	})
}

func (s *WarehouseService) ReceiveTransfer(ctx context.Context, transferID int64) error {
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		transfer, err := s.transferRepo.FindByIDForUpdate(tx, transferID)
		if err != nil {
			if stdErrors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New(40010, 404, "调拨单不存在")
			}
			return err
		}
		if transfer.Status != warehouseModel.TransferStatusShipped {
			return errors.New(40012, 400, "仅在途状态的调拨单可收货")
		}

		now := time.Now()
		for _, item := range transfer.Items {
			toLocID := int64(0)
			if item.ToLocationID != nil {
				toLocID = *item.ToLocationID
			}
			// 调入入库：沿用调出时确定的 UnitCost，保持跨仓成本传递与一致性
			_, err := s.ApplyInboundTx(ctx, tx, dto.MoveInput{
				MaterialID:  item.MaterialID,
				WarehouseID: transfer.ToWarehouseID,
				LocationID:  toLocID,
				Qty:         item.Qty,
				UnitCost:    item.UnitCost,
				BizType:     BizTypeLocationTransfer,
				BizNo:       transfer.TransferNo,
			})
			if err != nil {
				return err
			}
			item.ReceivedQty = item.Qty
			if err := s.transferRepo.UpdateItem(tx, item); err != nil {
				return err
			}
		}

		transfer.Status = warehouseModel.TransferStatusCompleted
		transfer.ReceivedAt = &now
		transfer.UpdatedAt = now

		return s.transferRepo.Update(tx, transfer)
	})
}

func (s *WarehouseService) CancelTransfer(ctx context.Context, transferID int64) error {
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		transfer, err := s.transferRepo.FindByIDForUpdate(tx, transferID)
		if err != nil {
			if stdErrors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New(40010, 404, "调拨单不存在")
			}
			return err
		}
		if transfer.Status == warehouseModel.TransferStatusShipped || transfer.Status == warehouseModel.TransferStatusCompleted {
			return errors.New(40013, 400, "在途或已完成的调拨单不能取消")
		}
		transfer.Status = warehouseModel.TransferStatusCancelled
		transfer.UpdatedAt = time.Now()
		return s.transferRepo.Update(tx, transfer)
	})
}

func (s *WarehouseService) GetTransfer(ctx context.Context, id int64) (*warehouseModel.InvTransfer, error) {
	transfer, err := s.transferRepo.FindByID(id)
	if err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(40010, 404, "调拨单不存在")
		}
		return nil, err
	}
	return transfer, nil
}

func (s *WarehouseService) ListTransfers(ctx context.Context, req dto.TransferQuery) ([]*warehouseModel.InvTransfer, int64, error) {
	return s.transferRepo.List(req.FromWarehouseID, req.ToWarehouseID, req.Status, req.TransferNo, req.Page, req.PageSize)
}

