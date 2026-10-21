// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	financeService "erp-system/internal/finance/service"
	mdmModel "erp-system/internal/mdm/model"
	mdmRepo "erp-system/internal/mdm/repository"
	"erp-system/internal/pkg/auth"
	"erp-system/internal/pkg/bizno"
	"erp-system/internal/pkg/datascope"
	"erp-system/internal/pkg/db"
	"erp-system/internal/pkg/decimalx"
	"erp-system/internal/pkg/errors"
	"erp-system/internal/pkg/idemp"
	"erp-system/internal/purchase/dto"
	"erp-system/internal/purchase/model"
	"erp-system/internal/purchase/repository"
	warehouseDto "erp-system/internal/warehouse/dto"
	warehouseService "erp-system/internal/warehouse/service"
)

const (
	// 状态码与库表 DEFAULT 统一（迁移 0010）：10 草稿 → 20 已审批 → 30 部分收货 → 40 已完成；50 已取消。
	OrderStatusDraft             = 10
	OrderStatusApproved          = 20
	OrderStatusPartiallyReceived = 30
	OrderStatusCompleted         = 40
	OrderStatusCanceled          = 50
)

// financeGateway 聚合采购流程需要的财务同事务能力：凭证过账 + 预算占用/释放。
// 由 *financeService.FinanceService 满足。
type financeGateway interface {
	financeService.Poster
	ReserveBudgetTx(ctx context.Context, tx *gorm.DB, budgetType string, amount decimal.Decimal) error
	ReleaseBudgetTx(ctx context.Context, tx *gorm.DB, budgetType string, amount decimal.Decimal) error
}

// DataScopeResolver 由 IAM service 满足（窄接口，避免反向依赖 iam 包）。
type DataScopeResolver interface {
	ResolveDataScope(ctx context.Context, ownerCol, deptCol string) (datascope.Params, error)
}

type PurchaseService struct {
	txManager        *db.TxManager
	orderRepo        repository.PurchaseOrderRepository
	inboundRepo      repository.PurchaseInboundRepository
	supplierRepo     mdmRepo.SupplierRepository
	materialRepo     mdmRepo.MaterialRepository
	warehouseRepo    mdmRepo.WarehouseRepository
	locationRepo     mdmRepo.LocationRepository
	warehouseService *warehouseService.WarehouseService
	finance          financeGateway
	numbers          *bizno.Generator
	idemp            interface {
		Acquire(ctx context.Context, scope, key string) error
	}
	scopeResolver DataScopeResolver // 可选：未装配时列表不做数据权限过滤（测试/内部调用）
}

// SetDataScopeResolver 装配数据权限解析器（main 中注入 IAMService）。
func (s *PurchaseService) SetDataScopeResolver(r DataScopeResolver) {
	s.scopeResolver = r
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
	finance financeGateway,
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
		finance:          finance,
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
	totalAmount := decimal.Zero

	items := make([]model.PurPurchaseOrderItem, 0, len(req.Items))
	for _, item := range req.Items {
		mat, err := s.materialRepo.FindByID(item.MaterialID)
		if err != nil {
			return nil, errors.New(40003, 404, "物料不存在")
		}
		_ = mat
		// 落库标度：数量 3 位、单价 4 位；行金额与订单总额按金额 2 位归整。
		q := decimalx.Qty(item.Qty)
		price := decimalx.Cost(item.Price)
		totalAmount = totalAmount.Add(decimalx.Money(q.Mul(price)))
		items = append(items, model.PurPurchaseOrderItem{
			MaterialID:  item.MaterialID,
			Qty:         q,
			ReceivedQty: decimal.Zero,
			Price:       price,
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
			DeptID:      auth.DeptIDFromContext(ctx),
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

		// 审批即采购承诺：在同事务内原子占用本期采购预算。
		// 本期未设预算则不强制；余额不足返回错误，随审批事务一并回滚。
		if err := s.finance.ReserveBudgetTx(ctx, tx, financeService.BudgetTypePurchase, order.TotalAmount); err != nil {
			return err
		}

		return s.orderRepo.UpdateOrderStatus(tx, order.ID, OrderStatusApproved, map[string]interface{}{
			"approved_at": time.Now(),
			"approved_by": auth.UserIDFromContext(ctx),
		})
	})
}

// CancelOrder 取消采购订单。
//   - 草稿(1)：尚未审批、未占用预算，直接取消；
//   - 已审批(2)/部分收货(5)：仅在零收货时允许取消，并在同事务内释放审批时占用的
//     采购预算（GREATEST 兜底，不会释放为负）；已有入库记录则拒绝（库存/应付已生成）；
//   - 已完成(3)/已取消(4)：不允许取消。
func (s *PurchaseService) CancelOrder(ctx context.Context, orderNo string) error {
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		order, err := s.orderRepo.FindByOrderNoForUpdate(tx, orderNo)
		if err != nil {
			return errors.New(40001, 404, "采购订单不存在")
		}

		switch order.Status {
		case OrderStatusDraft:
			return s.orderRepo.UpdateOrderStatus(tx, order.ID, OrderStatusCanceled, nil)
		case OrderStatusApproved, OrderStatusPartiallyReceived:
			items, err := s.orderRepo.FindItems(order.ID)
			if err != nil {
				return err
			}
			for _, it := range items {
				if it.ReceivedQty.IsPositive() {
					return errors.New(40007, 409, "订单已有入库记录，不能取消")
				}
			}
			// 零收货：回退审批时占用的预算，随取消事务一并提交/回滚。
			if err := s.finance.ReleaseBudgetTx(ctx, tx, financeService.BudgetTypePurchase, order.TotalAmount); err != nil {
				return err
			}
			return s.orderRepo.UpdateOrderStatus(tx, order.ID, OrderStatusCanceled, nil)
		default:
			return errors.New(40004, 409, "当前订单状态不允许取消")
		}
	})
}

func (s *PurchaseService) ListOrders(ctx context.Context, req dto.PurchaseOrderListReq) ([]*dto.PurchaseOrderVO, int64, error) {
	var scopes []func(*gorm.DB) *gorm.DB
	if s.scopeResolver != nil {
		params, err := s.scopeResolver.ResolveDataScope(ctx, "created_by", "dept_id")
		if err != nil {
			return nil, 0, err
		}
		scopes = append(scopes, datascope.Apply(params))
	}
	list, total, err := s.orderRepo.List(req.OrderNo, req.SupplierID, req.Status, req.DateFrom, req.DateTo, req.Page, req.PageSize, scopes...)
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
	case OrderStatusPartiallyReceived:
		statusDesc = "部分收货"
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
	var afterQty decimal.Decimal
	var remaining decimal.Decimal

	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		order, err := s.orderRepo.FindByOrderNoForUpdate(tx, req.OrderNo)
		if err != nil {
			return errors.New(40001, 404, "采购订单不存在")
		}
		if order.Status != OrderStatusApproved && order.Status != OrderStatusPartiallyReceived {
			return errors.New(40004, 409, "订单未审批或已结束，无法入库")
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
		// 入参数量按 NUMERIC(18,3) 归整，后续库存、超收、收齐判断全部使用该值。
		reqQty := decimalx.Qty(req.Qty)
		if locked.ReceivedQty.Add(reqQty).Cmp(locked.Qty) > 0 {
			return errors.New(40006, 400, "入库数量超出订单数量")
		}

		// 在同一事务内写库存与台账
		moveRes, err := s.warehouseService.ApplyInboundTx(ctx, tx, warehouseDto.MoveInput{
			MaterialID:  material.ID,
			WarehouseID: warehouse.ID,
			LocationID:  location.ID,
			Qty:         reqQty,
			UnitCost:    locked.Price,
			BizType:     warehouseService.BizTypePurchaseInbound,
			BizNo:       inboundNo,
		})
		if err != nil {
			return err
		}
		afterQty = moveRes.AfterQty
		remaining = locked.Qty.Sub(locked.ReceivedQty).Sub(reqQty)

		if err := s.orderRepo.UpdateItemReceivedQty(tx, locked.ID, reqQty); err != nil {
			return err
		}

		// 金额单一来源：库存移动返回的 cost_amount（已按金额标度归整），
		// 入库单、分录、成本卡共用，杜绝三处重复 price*qty 产生尾差。
		amount := moveRes.CostAmount
		inbound := model.PurPurchaseInbound{
			InboundNo:   inboundNo,
			OrderID:     order.ID,
			SupplierID:  order.SupplierID,
			WarehouseID: warehouse.ID,
			Status:      1,
			CostAmount:  amount,
			CreatedBy:   auth.UserIDFromContext(ctx),
		}
		if err := s.inboundRepo.Create(tx, &inbound); err != nil {
			return err
		}

		// 同事务过账：借 原材料（1403）/ 贷 应付账款（2202），并写材料成本卡。
		// 金额取库存移动的 cost_amount；任一分录失败随业务事务一并回滚。
		if err := s.finance.PostTx(ctx, tx, financeService.PostingInput{
			BizType:     financeService.BizTypePurchaseInbound,
			BizNo:       inboundNo,
			Description: fmt.Sprintf("采购入库 %s（订单 %s）", inboundNo, req.OrderNo),
			Lines: []financeService.PostingLine{
				{AccountCode: financeService.AccountRawMaterials, AccountName: "原材料", Debit: amount},
				{AccountCode: financeService.AccountsPayable, AccountName: "应付账款", Credit: amount},
			},
			CostCards: []financeService.CostCardInput{{
				ProductID:  material.ID,
				CostType:   financeService.CostTypeMaterial,
				Amount:     amount,
				SourceType: financeService.CostSourcePurchaseInbound,
				SourceID:   inbound.ID,
			}},
		}); err != nil {
			return err
		}

		// 全部明细收齐则置为已完成(3)，否则置为部分收货(5)。锁定行本次后的收货数
		// = locked.ReceivedQty+req.Qty，其余明细沿用 items 快照。
		allDone := true
		for _, it := range items {
			received := it.ReceivedQty
			if it.ID == locked.ID {
				received = locked.ReceivedQty.Add(reqQty)
			}
			if received.Cmp(it.Qty) < 0 {
				allDone = false
				break
			}
		}
		targetStatus := OrderStatusPartiallyReceived
		if allDone {
			targetStatus = OrderStatusCompleted
		}
		return s.orderRepo.UpdateOrderStatus(tx, order.ID, targetStatus, nil)
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
