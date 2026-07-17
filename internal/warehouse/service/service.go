// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	mdmModel "erp-system/internal/mdm/model"
	mdmRepo "erp-system/internal/mdm/repository"
	"erp-system/internal/warehouse/dto"
	warehouseModel "erp-system/internal/warehouse/model"
	"erp-system/internal/warehouse/repository"
	"erp-system/internal/pkg/db"
	"erp-system/internal/pkg/errors"
)

const (
	BizTypePurchaseInbound   = 1
	BizTypeProductionOutbound = 2
	BizTypeProductionInbound = 3
	BizTypeInventoryCheck    = 4
	BizTypeLocationTransfer  = 5
)

type WarehouseService struct {
	txManager       *db.TxManager
	invRepo         repository.InventoryRepository
	ledgerRepo      repository.StockLedgerRepository
	materialRepo    mdmRepo.MaterialRepository
	warehouseRepo   mdmRepo.WarehouseRepository
	locationRepo    mdmRepo.LocationRepository
}

func NewWarehouseService(
	txManager *db.TxManager,
	invRepo repository.InventoryRepository,
	ledgerRepo repository.StockLedgerRepository,
	materialRepo mdmRepo.MaterialRepository,
	warehouseRepo mdmRepo.WarehouseRepository,
	locationRepo mdmRepo.LocationRepository,
) *WarehouseService {
	return &WarehouseService{
		txManager:      txManager,
		invRepo:       invRepo,
		ledgerRepo:    ledgerRepo,
		materialRepo:  materialRepo,
		warehouseRepo: warehouseRepo,
		locationRepo:  locationRepo,
	}
}

func (s *WarehouseService) Inbound(ctx context.Context, req dto.InboundScanReq) (*dto.InboundScanRes, error) {
	material, err := s.materialRepo.FindByCode(req.MaterialCode)
	if err != nil {
		return nil, errors.New(30002, 404, "物料不存在")
	}

	warehouse := &mdmModel.MdmWarehouse{ID: 1}
	if req.WarehouseCode != "" {
		warehouse, err = s.warehouseRepo.FindByCode(req.WarehouseCode)
		if err != nil {
			return nil, errors.New(30002, 404, "仓库不存在")
		}
	}

	location, err := s.locationRepo.FindByCode(warehouse.ID, req.LocationCode)
	if err != nil {
		return nil, errors.New(30002, 404, "库位不存在")
	}

	inboundNo := fmt.Sprintf("IB%v", time.Now().Unix())

	var afterQty float64
	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		inv, err := s.invRepo.FindByMaterialWarehouse(material.ID, warehouse.ID, location.ID)
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		if inv == nil {
			inv = &warehouseModel.InvInventory{
				MaterialID:   material.ID,
				WarehouseID:  warehouse.ID,
				LocationID:   location.ID,
				Qty:          0,
				AvailableQty: 0,
				AvgCost:      0,
			}
		}

		if inv.Qty == 0 {
			inv.AvgCost = 0
		} else {
			inv.AvgCost = (inv.AvgCost*inv.Qty + 0) / (inv.Qty + req.Qty)
		}
		inv.Qty += req.Qty
		inv.AvailableQty += req.Qty
		inv.UpdatedAt = time.Now()

		if err := s.invRepo.Upsert(tx, inv); err != nil {
			return err
		}

		afterQty = inv.Qty

		return s.ledgerRepo.Append(tx, &warehouseModel.InvStockLedger{
			MaterialID:  material.ID,
			WarehouseID: warehouse.ID,
			BizType:     BizTypePurchaseInbound,
			BizNo:       inboundNo,
			ChangeQty:   req.Qty,
			AfterQty:    inv.Qty,
			CostAmount:  0,
		})
	})

	if err != nil {
		return nil, err
	}

	return &dto.InboundScanRes{
		InboundNo: inboundNo,
		Matched:   true,
		DiffQty:   0,
		AfterQty:  afterQty,
	}, nil
}

func (s *WarehouseService) Outbound(ctx context.Context, req dto.OutboundScanReq) (*dto.InboundScanRes, error) {
	material, err := s.materialRepo.FindByCode(req.MaterialCode)
	if err != nil {
		return nil, errors.New(30002, 404, "物料不存在")
	}

	warehouse := &mdmModel.MdmWarehouse{ID: 1}
	if req.WarehouseCode != "" {
		warehouse, err = s.warehouseRepo.FindByCode(req.WarehouseCode)
		if err != nil {
			return nil, errors.New(30002, 404, "仓库不存在")
		}
	}

	var afterQty float64
	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		var inv warehouseModel.InvInventory
		err := tx.Where("material_id = ? AND warehouse_id = ?", material.ID, warehouse.ID).First(&inv).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return errors.New(30001, 409, "库存不足")
			}
			return err
		}

		if inv.AvailableQty < req.Qty {
			return errors.New(30001, 409, "库存不足")
		}

		inv.Qty -= req.Qty
		inv.AvailableQty -= req.Qty
		inv.UpdatedAt = time.Now()

		if err := tx.Save(&inv).Error; err != nil {
			return err
		}

		afterQty = inv.Qty

		outboundNo := req.OutboundNo
		if outboundNo == "" {
			outboundNo = fmt.Sprintf("OB%v", time.Now().Unix())
		}

		return s.ledgerRepo.Append(tx, &warehouseModel.InvStockLedger{
			MaterialID:  material.ID,
			WarehouseID: warehouse.ID,
			BizType:     BizTypeProductionOutbound,
			BizNo:       outboundNo,
			ChangeQty:   -req.Qty,
			AfterQty:    inv.Qty,
			CostAmount:  inv.AvgCost * req.Qty,
		})
	})

	if err != nil {
		return nil, err
	}

	return &dto.InboundScanRes{
		InboundNo: req.OutboundNo,
		Matched:   true,
		DiffQty:   0,
		AfterQty:  afterQty,
	}, nil
}

func (s *WarehouseService) ListInventory(ctx context.Context, req dto.InventoryQuery) ([]dto.InventoryVO, int64, error) {
	list, total, err := s.invRepo.List(req.WarehouseID, req.MaterialCode, req.MaterialName, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}

	var vos []dto.InventoryVO
	for _, inv := range list {
		m, _ := s.materialRepo.FindByID(inv.MaterialID)
		w, _ := s.warehouseRepo.FindByID(inv.WarehouseID)
		l, _ := s.locationRepo.FindByID(inv.LocationID)

		materialCode := ""
		materialName := ""
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
	var vos []dto.StockAlertVO
	var inventories []warehouseModel.InvInventory
	if err := s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		return tx.Where("available_qty < 10").Find(&inventories).Error
	}); err != nil {
		return nil, err
	}

	for _, inv := range inventories {
		m, _ := s.materialRepo.FindByID(inv.MaterialID)
		w, _ := s.warehouseRepo.FindByID(inv.WarehouseID)

		materialCode := ""
		materialName := ""
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
