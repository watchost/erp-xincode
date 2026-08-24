// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"time"

	"gorm.io/gorm"

	mdmModel "erp-system/internal/mdm/model"
	mdmRepo "erp-system/internal/mdm/repository"
	"erp-system/internal/pkg/auth"
	"erp-system/internal/pkg/bizno"
	"erp-system/internal/pkg/db"
	"erp-system/internal/pkg/errors"
	"erp-system/internal/pkg/idemp"
	"erp-system/internal/purchase/dto"
	"erp-system/internal/purchase/model"
	"erp-system/internal/purchase/repository"
	warehouseDto "erp-system/internal/warehouse/dto"
	warehouseService "erp-system/internal/warehouse/service"
)

const (
	OrderStatusDraft     = 1
	OrderStatusApproved  = 2
	OrderStatusCompleted = 3
	OrderStatusCanceled  = 4
)

type PurchaseService struct {
	txManager        *db.TxManager
	orderRepo        repository.PurchaseOrderRepository
	inboundRepo      repository.PurchaseInboundRepository
	supplierRepo     mdmRepo.SupplierRepository
	materialRepo     mdmRepo.MaterialRepository
	warehouseRepo    mdmRepo.WarehouseRepository
	locationRepo     mdmRepo.LocationRepository
	warehouseService *warehouseService.WarehouseService
	numbers          *bizno.Generator
	idemp            interface {
		Acquire(ctx context.Context, scope, key string) error
	}
}

func NewPurchaseService(
	txManager *db.TxManager,
	orderRepo repository.PurchaseOrderRepository,
	inboundRepo repository.PurchaseInboundRepository,
	supplierRepo mdmRepo.SupplierRepository,
	materialRepo mdmRepo.MaterialRepository,
	warehouseRepo mdmRepo.WarehouseRepository,
	locationRepo mdmRepo.LocationRepository,
	warehouseService *warehouseService.WarehouseService,
	numbers *bizno.Generator,
	idempGuard interface {
		Acquire(ctx context.Context, scope, key string) error
	},
) *PurchaseService {
	return &PurchaseService{
		txManager:        txManager,
		orderRepo:        orderRepo,
		inboundRepo:      inboundRepo,
		supplierRepo:     supplierRepo,
		materialRepo:     materialRepo,
		warehouseRepo:    warehouseRepo,
		locationRepo:     locationRepo,
		warehouseService: warehouseService,
		numbers:          numbers,
		idemp:            idempGuard,
	}
}

func (s *PurchaseService) acquireIdemp(ctx context.Context, scope, key string) error {
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

func (s *PurchaseService) CreateOrder(ctx context.Context, req dto.CreatePurchaseOrderReq) (*dto.PurchaseOrderVO, error) {
	supplier, err := s.supplierRepo.FindByID(req.SupplierID)
	if err != nil {
		return nil, errors.New(40002, 404, "供应商不存在")
	}

	orderNo := s.numbers.Next(ctx, "PO")
	totalAmount := 0.0

	items := make([]model.PurPurchaseOrderItem, 0, len(req.Items))
	for _, item := range req.Items {
		mat, err := s.materialRepo.FindByID(item.MaterialID)
		if err != nil {
			return nil, errors.New(40003, 404, "物料不存在")
		}
		_ = mat
		totalAmount += item.Qty * item.Price
		items = append(items, model.PurPurchaseOrderItem{
			MaterialID:  item.MaterialID,
			Qty:         item.Qty,
			ReceivedQty: 0,
			Price:       item.Price,
		})
	}

	var order model.PurPurchaseOrder
	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		order = model.PurPurchaseOrder{
			OrderNo:     orderNo,
			SupplierID:  req.SupplierID,
			Status:      OrderStatusDraft,
			TotalAmount: totalAmount,
			CreatedBy:   auth.UserIDFromContext(ctx),
		}
		if req.PlanArriveAt != "" {
			if t, perr := time.Parse(time.RFC3339, req.PlanArriveAt); perr == nil {
				order.PlanArriveAt = t
			}
		}
		if err := s.orderRepo.Create(tx, &order); err != nil {
			return err
		}
		for i := range items {
			items[i].OrderID = order.ID
		}
		return s.orderRepo.CreateItems(tx, items)
	})
	if err != nil {
		return nil, err
	}
	return s.buildOrderVO(&order, items, supplier), nil
}

func (s *PurchaseService) ApproveOrder(ctx context.Context, orderNo string) error {
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		order, err := s.orderRepo.FindByOrderNoForUpdate(tx, orderNo)
		if err != nil {
			return errors.New(40001, 404, "采购订单不存在")
		}
		if order.Status != OrderStatusDraft {
			return errors.New(40004, 409, "订单状态不允许审批")
		}
		return s.orderRepo.UpdateOrderStatus(tx, order.ID, OrderStatusApproved, map[string]interface{}{
			"approved_at": time.Now(),
		})
	})
}

func (s *PurchaseService) ListOrders(ctx context.Context, req dto.PurchaseOrderListReq) ([]*dto.PurchaseOrderVO, int64, error) {
	list, total, err := s.orderRepo.List(req.OrderNo, req.SupplierID, req.Status, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}
	vos := make([]*dto.PurchaseOrderVO, 0, len(list))
	for i := range list {
		order := list[i]
		supplier, _ := s.supplierRepo.FindByID(order.SupplierID)
		items, _ := s.orderRepo.FindItems(order.ID)
		vos = append(vos, s.buildOrderVO(&order, items, supplier))
	}
	return vos, total, nil
}

func (s *PurchaseService) buildOrderVO(order *model.PurPurchaseOrder, items []model.PurPurchaseOrderItem, supplier *mdmModel.MdmSupplier) *dto.PurchaseOrderVO {
	supplierName := ""
	if supplier != nil {
		supplierName = supplier.Name
	}
	itemVOs := make([]dto.PurchaseOrderItemVO, 0, len(items))
	for _, item := range items {
		m, _ := s.materialRepo.FindByID(item.MaterialID)
		materialCode, materialName := "", ""
		if m != nil {
			materialCode = m.MaterialCode
			materialName = m.Name
		}
		itemVOs = append(itemVOs, dto.PurchaseOrderItemVO{
			ID:           item.ID,
			MaterialID:   item.MaterialID,
			MaterialCode: materialCode,
			MaterialName: materialName,
			Qty:          item.Qty,
			ReceivedQty:  item.ReceivedQty,
			Price:        item.Price,
		})
	}
	statusDesc := ""
	switch order.Status {
	case OrderStatusDraft:
		statusDesc = "草稿"
	case OrderStatusApproved:
		statusDesc = "已审批"
	case OrderStatusCompleted:
		statusDesc = "已完成"
	case OrderStatusCanceled:
		statusDesc = "已取消"
	}
	return &dto.PurchaseOrderVO{
		ID:           order.ID,
		OrderNo:      order.OrderNo,
		SupplierID:   order.SupplierID,
		SupplierName: supplierName,
		Status:       order.Status,
		StatusDesc:   statusDesc,
		TotalAmount:  order.TotalAmount,
		PlanArriveAt: order.PlanArriveAt.Format(time.RFC3339),
		CreatedBy:    order.CreatedBy,
		CreatedAt:    order.CreatedAt.Format(time.RFC3339),
		Items:        itemVOs,
	}
}

// InboundScan 采购入库。
// 修复 P0：之前库存写入（warehouse.Inbound 独立事务）与单据 received_qty 更新
// 拆成两个事务，任一失败都会造成账实不符。现在合并到同一个 WithTx：
//  1. 锁定订单与明细行（FOR UPDATE），在锁内校验超收；
//  2. 在同事务内调用 warehouse.ApplyInboundTx（内部再锁定库存行）；
//  3. 更新 received_qty、写入库单、必要时把订单置为已完成。
func (s *PurchaseService) InboundScan(ctx context.Context, req dto.PurchaseInboundScanReq) (*dto.PurchaseInboundScanRes, error) {
	if err := s.acquireIdemp(ctx, "purchase-inbound", req.IdempotencyKey); err != nil {
		return nil, err
	}

	material, err := s.materialRepo.FindByCode(req.MaterialCode)
	if err != nil {
		return nil, errors.New(40003, 404, "物料不存在")
	}
	warehouse, err := s.warehouseRepo.FindByCode(req.WarehouseCode)
	if err != nil {
		return nil, errors.New(30002, 404, "仓库不存在")
	}
	location, err := s.locationRepo.FindByCode(warehouse.ID, req.LocationCode)
	if err != nil {
		return nil, errors.New(30002, 404, "库位不存在")
	}

	inboundNo := s.numbers.Next(ctx, "IB")
	var afterQty float64
	var remaining float64

	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		order, err := s.orderRepo.FindByOrderNoForUpdate(tx, req.OrderNo)
		if err != nil {
			return errors.New(40001, 404, "采购订单不存在")
		}
		if order.Status != OrderStatusApproved {
			return errors.New(40004, 409, "订单未审批，无法入库")
		}

		items, err := s.orderRepo.FindItems(order.ID)
		if err != nil {
			return err
		}
		var matchedItem *model.PurPurchaseOrderItem
		for i := range items {
			if items[i].MaterialID == material.ID {
				matchedItem = &items[i]
				break
			}
		}
		if matchedItem == nil {
			return errors.New(40005, 400, "物料不在采购订单中")
		}

		// 锁定明细行后再校验，杜绝并发扫码超收
		locked, err := s.orderRepo.FindItemForUpdate(tx, matchedItem.ID)
		if err != nil {
			return err
		}
		if locked.ReceivedQty+req.Qty > locked.Qty {
			return errors.New(40006, 400, "入库数量超出订单数量")
		}

		// 在同一事务内写库存与台账
		moveRes, err := s.warehouseService.ApplyInboundTx(ctx, tx, warehouseDto.MoveInput{
			MaterialID:  material.ID,
			WarehouseID: warehouse.ID,
			LocationID:  location.ID,
			Qty:         req.Qty,
			UnitCost:    locked.Price,
			BizType:     warehouseService.BizTypePurchaseInbound,
			BizNo:       inboundNo,
		})
		if err != nil {
			return err
		}
		afterQty = moveRes.AfterQty
		remaining = locked.Qty - locked.ReceivedQty - req.Qty

		if err := s.orderRepo.UpdateItemReceivedQty(tx, locked.ID, req.Qty); err != nil {
			return err
		}

		inbound := model.PurPurchaseInbound{
			InboundNo:   inboundNo,
			OrderID:     order.ID,
			SupplierID:  order.SupplierID,
			WarehouseID: warehouse.ID,
			Status:      1,
			CostAmount:  locked.Price * req.Qty,
			CreatedBy:   auth.UserIDFromContext(ctx),
		}
		if err := s.inboundRepo.Create(tx, &inbound); err != nil {
			return err
		}

		// 全量收齐则置为已完成
		if locked.ReceivedQty+req.Qty >= locked.Qty {
			allDone := true
			for _, it := range items {
				if it.ID == locked.ID {
					if locked.ReceivedQty+req.Qty < it.Qty {
						allDone = false
					}
				} else if it.ReceivedQty < it.Qty {
					allDone = false
				}
			}
			if allDone {
				if err := s.orderRepo.UpdateOrderStatus(tx, order.ID, OrderStatusCompleted, nil); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &dto.PurchaseInboundScanRes{
		InboundNo: inboundNo,
		Matched:   true,
		DiffQty:   remaining,
		AfterQty:  afterQty,
	}, nil
}
