// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	mdmModel "erp-system/internal/mdm/model"
	mdmRepo "erp-system/internal/mdm/repository"
	"erp-system/internal/purchase/dto"
	"erp-system/internal/purchase/model"
	"erp-system/internal/purchase/repository"
	"erp-system/internal/pkg/db"
	"erp-system/internal/pkg/errors"
	warehouseService "erp-system/internal/warehouse/service"
	warehouseDto "erp-system/internal/warehouse/dto"
)

const (
	OrderStatusDraft    = 1
	OrderStatusApproved = 2
	OrderStatusCompleted = 3
)

type PurchaseService struct {
	txManager              *db.TxManager
	orderRepo              repository.PurchaseOrderRepository
	inboundRepo            repository.PurchaseInboundRepository
	supplierRepo           mdmRepo.SupplierRepository
	materialRepo           mdmRepo.MaterialRepository
	warehouseService       *warehouseService.WarehouseService
}

func NewPurchaseService(
	txManager *db.TxManager,
	orderRepo repository.PurchaseOrderRepository,
	inboundRepo repository.PurchaseInboundRepository,
	supplierRepo mdmRepo.SupplierRepository,
	materialRepo mdmRepo.MaterialRepository,
	warehouseService *warehouseService.WarehouseService,
) *PurchaseService {
	return &PurchaseService{
		txManager:        txManager,
		orderRepo:       orderRepo,
		inboundRepo:     inboundRepo,
		supplierRepo:    supplierRepo,
		materialRepo:    materialRepo,
		warehouseService: warehouseService,
	}
}

func (s *PurchaseService) CreateOrder(ctx context.Context, req dto.CreatePurchaseOrderReq) (*dto.PurchaseOrderVO, error) {
	supplier, err := s.supplierRepo.FindByID(req.SupplierID)
	if err != nil {
		return nil, errors.New(40002, 404, "供应商不存在")
	}

	orderNo := fmt.Sprintf("PO%v", time.Now().Unix())
	totalAmount := 0.0

	var items []model.PurPurchaseOrderItem
	for _, item := range req.Items {
		material, err := s.materialRepo.FindByID(item.MaterialID)
		if err != nil {
			return nil, errors.New(40003, 404, "物料不存在")
		}
		_ = material
		totalAmount += item.Qty * item.Price
		items = append(items, model.PurPurchaseOrderItem{
			OrderID:    0,
			MaterialID: item.MaterialID,
			Qty:        item.Qty,
			ReceivedQty: 0,
			Price:      item.Price,
		})
	}

	var order model.PurPurchaseOrder
	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		order = model.PurPurchaseOrder{
			OrderNo:      orderNo,
			SupplierID:   req.SupplierID,
			Status:       OrderStatusDraft,
			TotalAmount:  totalAmount,
			CreatedBy:    1,
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
	err := s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		order, err := s.orderRepo.FindByOrderNo(orderNo)
		if err != nil {
			return errors.New(40001, 404, "采购订单不存在")
		}
		if order.Status != OrderStatusDraft {
			return errors.New(40004, 409, "订单状态不允许审批")
		}
		return tx.Model(order).Update("status", OrderStatusApproved).Error
	})
	return err
}

func (s *PurchaseService) ListOrders(ctx context.Context, req dto.PurchaseOrderListReq) ([]*dto.PurchaseOrderVO, int64, error) {
	list, total, err := s.orderRepo.List(req.OrderNo, req.SupplierID, req.Status, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}

	var vos []*dto.PurchaseOrderVO
	for _, order := range list {
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

	var itemVOs []dto.PurchaseOrderItemVO
	for _, item := range items {
		m, _ := s.materialRepo.FindByID(item.MaterialID)
		materialCode := ""
		materialName := ""
		if m != nil {
			materialCode = m.MaterialCode
			materialName = m.Name
		}
		itemVOs = append(itemVOs, dto.PurchaseOrderItemVO{
			ID:          item.ID,
			MaterialID:  item.MaterialID,
			MaterialCode: materialCode,
			MaterialName: materialName,
			Qty:         item.Qty,
			ReceivedQty: item.ReceivedQty,
			Price:       item.Price,
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
	}

	return &dto.PurchaseOrderVO{
		ID:            order.ID,
		OrderNo:       order.OrderNo,
		SupplierID:    order.SupplierID,
		SupplierName:  supplierName,
		Status:        order.Status,
		StatusDesc:    statusDesc,
		TotalAmount:   order.TotalAmount,
		CreatedBy:     order.CreatedBy,
		CreatedAt:     order.CreatedAt.Format(time.RFC3339),
		Items:         itemVOs,
	}
}

func (s *PurchaseService) InboundScan(ctx context.Context, req dto.PurchaseInboundScanReq) (*dto.PurchaseInboundScanRes, error) {
	order, err := s.orderRepo.FindByOrderNo(req.OrderNo)
	if err != nil {
		return nil, errors.New(40001, 404, "采购订单不存在")
	}
	if order.Status != OrderStatusApproved {
		return nil, errors.New(40004, 409, "订单未审批，无法入库")
	}

	items, err := s.orderRepo.FindItems(order.ID)
	if err != nil {
		return nil, err
	}

	material, err := s.materialRepo.FindByCode(req.MaterialCode)
	if err != nil {
		return nil, errors.New(40003, 404, "物料不存在")
	}

	var matchedItem *model.PurPurchaseOrderItem
	for i := range items {
		if items[i].MaterialID == material.ID {
			matchedItem = &items[i]
			break
		}
	}
	if matchedItem == nil {
		return nil, errors.New(40005, 400, "物料不在采购订单中")
	}

	if matchedItem.ReceivedQty+req.Qty > matchedItem.Qty {
		return nil, errors.New(40006, 400, "入库数量超出订单数量")
	}

	inboundReq := warehouseDto.InboundScanReq{
		MaterialCode: req.MaterialCode,
		LocationCode: req.LocationCode,
		Qty:          req.Qty,
	}

	warehouseRes, err := s.warehouseService.Inbound(ctx, inboundReq)
	if err != nil {
		return nil, err
	}

	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		if err := s.orderRepo.UpdateItemReceivedQty(tx, matchedItem.ID, req.Qty); err != nil {
			return err
		}

		inbound := model.PurPurchaseInbound{
			InboundNo:   warehouseRes.InboundNo,
			OrderID:     order.ID,
			SupplierID:  order.SupplierID,
			WarehouseID: 1,
			Status:      1,
			CostAmount:  matchedItem.Price * req.Qty,
			CreatedBy:   1,
		}
		return s.inboundRepo.Create(tx, &inbound)
	})

	if err != nil {
		return nil, err
	}

	return &dto.PurchaseInboundScanRes{
		InboundNo: warehouseRes.InboundNo,
		Matched:   true,
		DiffQty:   matchedItem.Qty - matchedItem.ReceivedQty - req.Qty,
		AfterQty:  warehouseRes.AfterQty,
	}, nil
}
