// Copyright 2026 zhouhouping. All Rights Reserved.

// Package service 销售管理（P2 §3.1）：销售订单/出库/退货，与采购模块对称。
//
// 核心事务（出库扫码）：锁定订单与明细行 → 超发校验 → 同事务库存扣减+台账
// （warehouse.ApplyOutboundTx，移动加权成本结转）→ 更新 shipped_qty → 写出库单 →
// 同事务过账两张凭证（成本：借 6401/贷 1405；应收：借 1122/贷 6001）。
// 任何一步失败整事务回滚，保证库存/单据/财务严格一致。
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
	"erp-system/internal/sales/dto"
	"erp-system/internal/sales/model"
	"erp-system/internal/sales/repository"
	warehouseDto "erp-system/internal/warehouse/dto"
	warehouseService "erp-system/internal/warehouse/service"
)

const (
	// 状态码与采购对称：10 草稿 → 20 已审批 → 30 部分发货 → 40 已完成；50 已取消。
	OrderStatusDraft            = 10
	OrderStatusApproved         = 20
	OrderStatusPartiallyShipped = 30
	OrderStatusCompleted        = 40
	OrderStatusCanceled         = 50
)

// 退货单状态：10 待入库 → 20 已入库 → 50 已取消。
const (
	ReturnStatusPending  = 10
	ReturnStatusReceived = 20
	ReturnStatusCanceled = 50
)

// DataScopeResolver 由 IAM service 满足（窄接口，避免反向依赖 iam 包）。
type DataScopeResolver interface {
	ResolveDataScope(ctx context.Context, ownerCol, deptCol string) (datascope.Params, error)
}

type SalesService struct {
	txManager        *db.TxManager
	orderRepo        repository.SalesOrderRepository
	outboundRepo     repository.SalesOutboundRepository
	returnRepo       repository.SalesReturnRepository
	customerRepo     mdmRepo.CustomerRepository
	materialRepo     mdmRepo.MaterialRepository
	warehouseRepo    mdmRepo.WarehouseRepository
	locationRepo     mdmRepo.LocationRepository
	warehouseService *warehouseService.WarehouseService
	finance          financeService.Poster
	numbers          *bizno.Generator
	idemp            interface {
		Acquire(ctx context.Context, scope, key string) error
	}
	scopeResolver DataScopeResolver // 可选：未装配时列表不做数据权限过滤（测试/内部调用）
}

// SetDataScopeResolver 装配数据权限解析器（main 中注入 IAMService）。
func (s *SalesService) SetDataScopeResolver(r DataScopeResolver) {
	s.scopeResolver = r
}

func NewSalesService(
	txManager *db.TxManager,
	orderRepo repository.SalesOrderRepository,
	outboundRepo repository.SalesOutboundRepository,
	returnRepo repository.SalesReturnRepository,
	customerRepo mdmRepo.CustomerRepository,
	materialRepo mdmRepo.MaterialRepository,
	warehouseRepo mdmRepo.WarehouseRepository,
	locationRepo mdmRepo.LocationRepository,
	warehouseService *warehouseService.WarehouseService,
	finance financeService.Poster,
	numbers *bizno.Generator,
	idempGuard interface {
		Acquire(ctx context.Context, scope, key string) error
	},
) *SalesService {
	return &SalesService{
		txManager:        txManager,
		orderRepo:        orderRepo,
		outboundRepo:     outboundRepo,
		returnRepo:       returnRepo,
		customerRepo:     customerRepo,
		materialRepo:     materialRepo,
		warehouseRepo:    warehouseRepo,
		locationRepo:     locationRepo,
		warehouseService: warehouseService,
		finance:          finance,
		numbers:          numbers,
		idemp:            idempGuard,
	}
}

func (s *SalesService) acquireIdemp(ctx context.Context, scope, key string) error {
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

// buildItems 服务端统一定价：amount = qty*price*(1-discount)，总额按金额标度归整。
// 返回明细、订单总额、折扣总额（原价合计 − 折后合计）。
func (s *SalesService) buildItems(reqs []dto.SalesOrderItemReq) ([]model.SalSalesOrderItem, decimal.Decimal, decimal.Decimal, error) {
	items := make([]model.SalSalesOrderItem, 0, len(reqs))
	totalAmount := decimal.Zero
	grossAmount := decimal.Zero
	for _, item := range reqs {
		if _, err := s.materialRepo.FindByID(item.MaterialID); err != nil {
			return nil, decimal.Zero, decimal.Zero, errors.New(70003, 404, "物料不存在")
		}
		q := decimalx.Qty(item.Qty)
		price := decimalx.Money(item.Price)
		discount := decimalx.Rate(item.Discount)
		grossAmount = grossAmount.Add(decimalx.Money(q.Mul(price)))
		amount := decimalx.Money(q.Mul(price).Mul(decimal.NewFromInt(1).Sub(discount)))
		totalAmount = totalAmount.Add(amount)
		items = append(items, model.SalSalesOrderItem{
			MaterialID: item.MaterialID,
			Qty:        q,
			ShippedQty: decimal.Zero,
			Price:      price,
			Discount:   discount,
			Amount:     amount,
		})
	}
	return items, totalAmount, grossAmount.Sub(totalAmount), nil
}

// checkCreditLimit 客户信用额度校验：已用额度（已审批未完成订单合计）+ 本单金额 ≤ credit_limit。
// credit_limit = 0 视为不限额（主数据默认 0，未做授信的客户不阻断下单）。
// TODO(P2 §3.5)：应收应付落地后改为「未核销应收余额 + 本单金额」口径。
func (s *SalesService) checkCreditLimit(db *gorm.DB, customer *mdmModel.MdmCustomer, amount decimal.Decimal) error {
	if !customer.CreditLimit.IsPositive() {
		return nil
	}
	used, err := s.orderRepo.SumOccupiedAmountTx(db, customer.ID)
	if err != nil {
		return err
	}
	if used.Add(amount).Cmp(customer.CreditLimit) > 0 {
		return errors.New(70008, 409, fmt.Sprintf("超出客户信用额度（额度 %s，已占用 %s，本单 %s）",
			customer.CreditLimit, used, amount))
	}
	return nil
}

func (s *SalesService) CreateOrder(ctx context.Context, req dto.CreateSalesOrderReq) (*dto.SalesOrderVO, error) {
	customer, err := s.customerRepo.FindByID(req.CustomerID)
	if err != nil {
		return nil, errors.New(70002, 404, "客户不存在")
	}
	if customer.Status != 1 {
		return nil, errors.New(70002, 409, "客户已停用，不能下单")
	}

	items, totalAmount, discountAmount, err := s.buildItems(req.Items)
	if err != nil {
		return nil, err
	}

	orderNo := s.numbers.Next(ctx, "SO")
	var order model.SalSalesOrder
	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		// 信用额度校验放进建单事务，与订单写入同一读口径。
		if err := s.checkCreditLimit(tx, customer, totalAmount); err != nil {
			return err
		}
		order = model.SalSalesOrder{
			OrderNo:        orderNo,
			CustomerID:     req.CustomerID,
			Status:         OrderStatusDraft,
			TotalAmount:    totalAmount,
			DiscountAmount: discountAmount,
			Remark:         req.Remark,
			CreatedBy:      auth.UserIDFromContext(ctx),
			DeptID:         auth.DeptIDFromContext(ctx),
		}
		if req.PlanDeliveryAt != "" {
			t, perr := time.Parse(time.RFC3339, req.PlanDeliveryAt)
			if perr != nil {
				return errors.New(10003, 400, "plan_delivery_at 格式错误（RFC3339）")
			}
			order.PlanDeliveryAt = &t
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
	return s.buildOrderVO(&order, items, customer), nil
}

// UpdateOrder 仅草稿状态可修改（整体替换明细），审批后明细冻结。
func (s *SalesService) UpdateOrder(ctx context.Context, orderNo string, req dto.UpdateSalesOrderReq) (*dto.SalesOrderVO, error) {
	items, totalAmount, discountAmount, err := s.buildItems(req.Items)
	if err != nil {
		return nil, err
	}

	var order *model.SalSalesOrder
	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		o, err := s.orderRepo.FindByOrderNoForUpdate(tx, orderNo)
		if err != nil {
			return errors.New(70001, 404, "销售订单不存在")
		}
		if o.Status != OrderStatusDraft {
			return errors.New(70004, 409, "仅草稿状态的订单可以修改")
		}
		if err := s.orderRepo.DeleteItems(tx, o.ID); err != nil {
			return err
		}
		for i := range items {
			items[i].OrderID = o.ID
		}
		if err := s.orderRepo.CreateItems(tx, items); err != nil {
			return err
		}
		fields := map[string]interface{}{
			"total_amount":    totalAmount,
			"discount_amount": discountAmount,
			"remark":          req.Remark,
		}
		if req.PlanDeliveryAt != "" {
			t, perr := time.Parse(time.RFC3339, req.PlanDeliveryAt)
			if perr != nil {
				return errors.New(10003, 400, "plan_delivery_at 格式错误（RFC3339）")
			}
			fields["plan_delivery_at"] = t
		}
		if err := tx.Model(&model.SalSalesOrder{}).Where("id = ?", o.ID).Updates(fields).Error; err != nil {
			return err
		}
		order = o
		order.TotalAmount = totalAmount
		order.DiscountAmount = discountAmount
		order.Remark = req.Remark
		return nil
	})
	if err != nil {
		return nil, err
	}
	customer, _ := s.customerRepo.FindByID(order.CustomerID)
	return s.buildOrderVO(order, items, customer), nil
}

func (s *SalesService) ApproveOrder(ctx context.Context, orderNo string) error {
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		order, err := s.orderRepo.FindByOrderNoForUpdate(tx, orderNo)
		if err != nil {
			return errors.New(70001, 404, "销售订单不存在")
		}
		if order.Status != OrderStatusDraft {
			return errors.New(70004, 409, "订单状态不允许审批")
		}
		// 审批即销售承诺：在锁内按最新占用复检信用额度（下单后额度可能被其他单占满）。
		customer, err := s.customerRepo.FindByID(order.CustomerID)
		if err != nil {
			return errors.New(70002, 404, "客户不存在")
		}
		if err := s.checkCreditLimit(tx, customer, order.TotalAmount); err != nil {
			return err
		}

		now := time.Now()
		uid := auth.UserIDFromContext(ctx)
		return s.orderRepo.UpdateOrderStatus(tx, order.ID, OrderStatusApproved, map[string]interface{}{
			"approved_at": now,
			"approved_by": uid,
		})
	})
}

// CancelOrder 取消销售订单。
//   - 草稿：直接取消；
//   - 已审批/部分发货：仅在零发货时允许取消；已有出库记录则拒绝（库存/应收已生成）；
//   - 已完成/已取消：不允许。
func (s *SalesService) CancelOrder(ctx context.Context, orderNo string) error {
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		order, err := s.orderRepo.FindByOrderNoForUpdate(tx, orderNo)
		if err != nil {
			return errors.New(70001, 404, "销售订单不存在")
		}

		switch order.Status {
		case OrderStatusDraft:
			return s.orderRepo.UpdateOrderStatus(tx, order.ID, OrderStatusCanceled, nil)
		case OrderStatusApproved, OrderStatusPartiallyShipped:
			items, err := s.orderRepo.FindItemsTx(tx, order.ID)
			if err != nil {
				return err
			}
			for _, it := range items {
				if it.ShippedQty.IsPositive() {
					return errors.New(70007, 409, "订单已有出库记录，不能取消")
				}
			}
			return s.orderRepo.UpdateOrderStatus(tx, order.ID, OrderStatusCanceled, nil)
		default:
			return errors.New(70004, 409, "当前订单状态不允许取消")
		}
	})
}

func (s *SalesService) GetOrder(ctx context.Context, orderNo string) (*dto.SalesOrderVO, error) {
	order, err := s.orderRepo.FindByOrderNo(orderNo)
	if err != nil {
		return nil, errors.New(70001, 404, "销售订单不存在")
	}
	items, err := s.orderRepo.FindItems(order.ID)
	if err != nil {
		return nil, err
	}
	customer, _ := s.customerRepo.FindByID(order.CustomerID)
	return s.buildOrderVO(order, items, customer), nil
}

func (s *SalesService) ListOrders(ctx context.Context, req dto.SalesOrderListReq) ([]*dto.SalesOrderVO, int64, error) {
	var scopes []func(*gorm.DB) *gorm.DB
	if s.scopeResolver != nil {
		params, err := s.scopeResolver.ResolveDataScope(ctx, "created_by", "dept_id")
		if err != nil {
			return nil, 0, err
		}
		scopes = append(scopes, datascope.Apply(params))
	}
	list, total, err := s.orderRepo.List(req.OrderNo, req.CustomerID, req.Status, req.DateFrom, req.DateTo, req.Page, req.PageSize, scopes...)
	if err != nil {
		return nil, 0, err
	}
	vos := make([]*dto.SalesOrderVO, 0, len(list))
	for i := range list {
		order := list[i]
		customer, _ := s.customerRepo.FindByID(order.CustomerID)
		items, _ := s.orderRepo.FindItems(order.ID)
		vos = append(vos, s.buildOrderVO(&order, items, customer))
	}
	return vos, total, nil
}

func (s *SalesService) buildOrderVO(order *model.SalSalesOrder, items []model.SalSalesOrderItem, customer *mdmModel.MdmCustomer) *dto.SalesOrderVO {
	customerName := ""
	if customer != nil {
		customerName = customer.Name
	}
	itemVOs := make([]dto.SalesOrderItemVO, 0, len(items))
	for _, item := range items {
		m, _ := s.materialRepo.FindByID(item.MaterialID)
		materialCode, materialName := "", ""
		if m != nil {
			materialCode = m.MaterialCode
			materialName = m.Name
		}
		itemVOs = append(itemVOs, dto.SalesOrderItemVO{
			ID:           item.ID,
			MaterialID:   item.MaterialID,
			MaterialCode: materialCode,
			MaterialName: materialName,
			Qty:          item.Qty,
			ShippedQty:   item.ShippedQty,
			Price:        item.Price,
			Discount:     item.Discount,
			Amount:       item.Amount,
		})
	}
	planDelivery := ""
	if order.PlanDeliveryAt != nil {
		planDelivery = order.PlanDeliveryAt.Format(time.RFC3339)
	}
	return &dto.SalesOrderVO{
		ID:             order.ID,
		OrderNo:        order.OrderNo,
		CustomerID:     order.CustomerID,
		CustomerName:   customerName,
		Status:         order.Status,
		StatusDesc:     OrderStatusDesc(order.Status),
		TotalAmount:    order.TotalAmount,
		DiscountAmount: order.DiscountAmount,
		PlanDeliveryAt: planDelivery,
		Remark:         order.Remark,
		CreatedBy:      order.CreatedBy,
		CreatedAt:      order.CreatedAt.Format(time.RFC3339),
		Items:          itemVOs,
	}
}

// OrderStatusDesc 订单状态文案（dashboard 等展示复用）。
func OrderStatusDesc(status int) string {
	switch status {
	case OrderStatusDraft:
		return "草稿"
	case OrderStatusApproved:
		return "已审批"
	case OrderStatusPartiallyShipped:
		return "部分发货"
	case OrderStatusCompleted:
		return "已完成"
	case OrderStatusCanceled:
		return "已取消"
	}
	return ""
}

// OutboundScan 销售出库扫码（核心事务，与采购入库对称）：
//  1. 锁定订单与明细行（FOR UPDATE），在锁内校验超发；
//  2. 同事务扣库存+台账（ApplyOutboundTx，按移动加权平均结转成本）；
//  3. 更新 shipped_qty、写出库单/明细（记录发货时点 unit_cost 供退货回填）；
//  4. 同事务过账：成本凭证 借 6401 主营业务成本/贷 1405 库存商品，
//     应收凭证 借 1122 应收账款/贷 6001 主营业务收入；
//  5. 全部明细发齐则订单置已完成，否则部分发货。
func (s *SalesService) OutboundScan(ctx context.Context, req dto.SalesOutboundScanReq) (*dto.SalesOutboundScanRes, error) {
	if err := s.acquireIdemp(ctx, "sales-outbound", req.IdempotencyKey); err != nil {
		return nil, err
	}

	material, err := s.materialRepo.FindByCode(req.MaterialCode)
	if err != nil {
		return nil, errors.New(70003, 404, "物料不存在")
	}
	warehouse, err := s.warehouseRepo.FindByCode(req.WarehouseCode)
	if err != nil {
		return nil, errors.New(30002, 404, "仓库不存在")
	}
	location, err := s.locationRepo.FindByCode(warehouse.ID, req.LocationCode)
	if err != nil {
		return nil, errors.New(30002, 404, "库位不存在")
	}

	outboundNo := s.numbers.Next(ctx, "OB")
	var afterQty, remaining, costAmount decimal.Decimal

	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		order, err := s.orderRepo.FindByOrderNoForUpdate(tx, req.OrderNo)
		if err != nil {
			return errors.New(70001, 404, "销售订单不存在")
		}
		if order.Status != OrderStatusApproved && order.Status != OrderStatusPartiallyShipped {
			return errors.New(70004, 409, "订单未审批或已结束，无法出库")
		}

		items, err := s.orderRepo.FindItemsTx(tx, order.ID)
		if err != nil {
			return err
		}
		var matchedItem *model.SalSalesOrderItem
		for i := range items {
			if items[i].MaterialID == material.ID {
				matchedItem = &items[i]
				break
			}
		}
		if matchedItem == nil {
			return errors.New(70005, 400, "物料不在销售订单中")
		}

		// 锁定明细行后再校验，杜绝并发扫码超发
		locked, err := s.orderRepo.FindItemForUpdate(tx, matchedItem.ID)
		if err != nil {
			return err
		}
		reqQty := decimalx.Qty(req.Qty)
		if locked.ShippedQty.Add(reqQty).Cmp(locked.Qty) > 0 {
			return errors.New(70006, 400, "出库数量超出订单数量")
		}

		// 同一事务内扣库存与台账（按移动加权平均成本结转）
		moveRes, err := s.warehouseService.ApplyOutboundTx(ctx, tx, warehouseDto.MoveInput{
			MaterialID:  material.ID,
			WarehouseID: warehouse.ID,
			LocationID:  location.ID,
			Qty:         reqQty,
			BizType:     warehouseService.BizTypeSalesOutbound,
			BizNo:       outboundNo,
		})
		if err != nil {
			return err
		}
		afterQty = moveRes.AfterQty
		costAmount = moveRes.CostAmount
		remaining = locked.Qty.Sub(locked.ShippedQty).Sub(reqQty)

		if err := s.orderRepo.UpdateItemShippedQty(tx, locked.ID, reqQty); err != nil {
			return err
		}

		outbound := model.SalSalesOutbound{
			OutboundNo:  outboundNo,
			OrderID:     order.ID,
			CustomerID:  order.CustomerID,
			WarehouseID: warehouse.ID,
			Status:      1,
			CostAmount:  costAmount,
			CreatedBy:   auth.UserIDFromContext(ctx),
		}
		if req.IdempotencyKey != "" {
			outbound.IdempotencyKey = &req.IdempotencyKey
		}
		if err := s.outboundRepo.Create(tx, &outbound); err != nil {
			return err
		}
		if err := s.outboundRepo.CreateItems(tx, []model.SalSalesOutboundItem{{
			OutboundID: outbound.ID,
			MaterialID: material.ID,
			Qty:        reqQty,
			UnitCost:   moveRes.AvgCost,
			CostAmount: costAmount,
		}}); err != nil {
			return err
		}

		uid := auth.UserIDFromContext(ctx)
		// 成本结转：借 主营业务成本（6401）/ 贷 库存商品（1405）。
		// 金额单一来源：库存移动返回的 cost_amount（已按金额标度归整）。
		if costAmount.IsPositive() {
			if err := s.finance.PostTx(ctx, tx, financeService.PostingInput{
				BizType:     financeService.BizTypeSalesOutbound,
				BizNo:       outboundNo,
				Description: fmt.Sprintf("销售出库 %s（订单 %s）成本结转", outboundNo, req.OrderNo),
				CreatedBy:   uid,
				Lines: []financeService.PostingLine{
					{AccountCode: financeService.AccountMainCost, AccountName: "主营业务成本", Debit: costAmount},
					{AccountCode: financeService.AccountFinishedGoods, AccountName: "库存商品", Credit: costAmount},
				},
			}); err != nil {
				return err
			}
		}

		// 确认应收：借 应收账款（1122）/ 贷 主营业务收入（6001）。
		// 收入口径与订单行一致：qty*price*(1-discount)，按金额标度归整。
		// TODO(P2 §3.5)：同步生成 fin_receivable 应收单，税率/应交税费随应收模块落地。
		revenue := decimalx.Money(reqQty.Mul(locked.Price).Mul(decimal.NewFromInt(1).Sub(locked.Discount)))
		if revenue.IsPositive() {
			if err := s.finance.PostTx(ctx, tx, financeService.PostingInput{
				BizType:     financeService.BizTypeSalesOutbound,
				BizNo:       outboundNo,
				Description: fmt.Sprintf("销售出库 %s（订单 %s）确认应收", outboundNo, req.OrderNo),
				CreatedBy:   uid,
				Lines: []financeService.PostingLine{
					{AccountCode: financeService.AccountReceivable, AccountName: "应收账款", Debit: revenue},
					{AccountCode: financeService.AccountMainRevenue, AccountName: "主营业务收入", Credit: revenue},
				},
			}); err != nil {
				return err
			}
		}

		// 全部明细发齐则置已完成，否则部分发货。锁定行本次后的发货数
		// = locked.ShippedQty+reqQty，其余明细沿用 items 快照。
		allDone := true
		for _, it := range items {
			shipped := it.ShippedQty
			if it.ID == locked.ID {
				shipped = locked.ShippedQty.Add(reqQty)
			}
			if shipped.Cmp(it.Qty) < 0 {
				allDone = false
				break
			}
		}
		targetStatus := OrderStatusPartiallyShipped
		if allDone {
			targetStatus = OrderStatusCompleted
		}
		return s.orderRepo.UpdateOrderStatus(tx, order.ID, targetStatus, nil)
	})
	if err != nil {
		return nil, err
	}

	return &dto.SalesOutboundScanRes{
		OutboundNo: outboundNo,
		Matched:    true,
		DiffQty:    remaining,
		AfterQty:   afterQty,
		CostAmount: costAmount,
	}, nil
}

// ListOutbounds 出库单列表（可按订单过滤）。
func (s *SalesService) ListOutbounds(ctx context.Context, req dto.SalesOutboundListReq) ([]*dto.SalesOutboundVO, int64, error) {
	var orderID int64
	if req.OrderNo != "" {
		order, err := s.orderRepo.FindByOrderNo(req.OrderNo)
		if err != nil {
			return nil, 0, errors.New(70001, 404, "销售订单不存在")
		}
		orderID = order.ID
	}
	list, total, err := s.outboundRepo.List(orderID, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}
	vos := make([]*dto.SalesOutboundVO, 0, len(list))
	for i := range list {
		ob := list[i]
		vo := &dto.SalesOutboundVO{
			ID:          ob.ID,
			OutboundNo:  ob.OutboundNo,
			OrderID:     ob.OrderID,
			CustomerID:  ob.CustomerID,
			WarehouseID: ob.WarehouseID,
			CostAmount:  ob.CostAmount,
			CreatedBy:   ob.CreatedBy,
			CreatedAt:   ob.CreatedAt.Format(time.RFC3339),
		}
		if ob.OrderID > 0 {
			if o, err := s.orderRepo.FindByID(ob.OrderID); err == nil {
				vo.OrderNo = o.OrderNo
			}
		}
		if c, err := s.customerRepo.FindByID(ob.CustomerID); err == nil {
			vo.CustomerName = c.Name
		}
		vos = append(vos, vo)
	}
	return vos, total, nil
}


// CreateReturn 退货申请：引用原出库单，逐行校验「退货数 ≤ 出库数 − 已退数」
// （锁在校验在 ReceiveReturn 入库事务内复查，这里只做申请期前置校验）。
func (s *SalesService) CreateReturn(ctx context.Context, req dto.CreateSalesReturnReq) (*dto.SalesReturnVO, error) {
	outbound, err := s.outboundRepo.FindByOutboundNo(req.OutboundNo)
	if err != nil {
		return nil, errors.New(70009, 404, "出库单不存在")
	}
	warehouse, err := s.warehouseRepo.FindByCode(req.WarehouseCode)
	if err != nil {
		return nil, errors.New(30002, 404, "仓库不存在")
	}
	if _, err := s.locationRepo.FindByCode(warehouse.ID, req.LocationCode); err != nil {
		return nil, errors.New(30002, 404, "库位不存在")
	}

	order, err := s.orderRepo.FindByID(outbound.OrderID)
	if err != nil {
		return nil, errors.New(70001, 404, "关联销售订单不存在")
	}
	orderItems, err := s.orderRepo.FindItems(order.ID)
	if err != nil {
		return nil, err
	}
	outboundItems, err := s.outboundRepo.FindItems(outbound.ID)
	if err != nil {
		return nil, err
	}

	returnNo := s.numbers.Next(ctx, "SR")
	var ret model.SalSalesReturn
	var retItems []model.SalSalesReturnItem
	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		totalAmount := decimal.Zero
		retItems = make([]model.SalSalesReturnItem, 0, len(req.Items))
		for _, ri := range req.Items {
			// 找该物料在此出库单中的出库行（按出库行累计退货，防止跨单/超量退货）
			var obItem *model.SalSalesOutboundItem
			for i := range outboundItems {
				if outboundItems[i].MaterialID == ri.MaterialID {
					obItem = &outboundItems[i]
					break
				}
			}
			if obItem == nil {
				return errors.New(70010, 400, "退货物料不在原出库单中")
			}
			locked, err := s.outboundRepo.FindItemForUpdate(tx, obItem.ID)
			if err != nil {
				return err
			}
			qty := decimalx.Qty(ri.Qty)
			if locked.ReturnedQty.Add(qty).Cmp(locked.Qty) > 0 {
				return errors.New(70010, 400, "退货数量超出原出库数量")
			}
			// 申请即占用退货额度（returned_qty 在申请时累计而非入库时）——
			// 否则两张待入库退货单可合计超过出库数量，入库时才爆雷。
			if err := s.outboundRepo.UpdateItemReturnedQty(tx, locked.ID, qty); err != nil {
				return err
			}
			// 退货单价取订单行折后单价（应收冲减口径与确认时一致）
			price := decimal.Zero
			for _, oi := range orderItems {
				if oi.MaterialID == ri.MaterialID {
					price = decimalx.Money(oi.Price.Mul(decimal.NewFromInt(1).Sub(oi.Discount)))
					break
				}
			}
			amount := decimalx.Money(qty.Mul(price))
			totalAmount = totalAmount.Add(amount)
			retItems = append(retItems, model.SalSalesReturnItem{
				OutboundItemID: locked.ID,
				MaterialID:     ri.MaterialID,
				Qty:            qty,
				Price:          price,
				Amount:         amount,
			})
		}

		ret = model.SalSalesReturn{
			ReturnNo:    returnNo,
			OutboundID:  outbound.ID,
			CustomerID:  outbound.CustomerID,
			WarehouseID: warehouse.ID,
			Status:      ReturnStatusPending,
			TotalAmount: totalAmount,
			Reason:      req.Reason,
			CreatedBy:   auth.UserIDFromContext(ctx),
		}
		if err := s.returnRepo.Create(tx, &ret); err != nil {
			return err
		}
		for i := range retItems {
			retItems[i].ReturnID = ret.ID
		}
		return s.returnRepo.CreateItems(tx, retItems)
	})
	if err != nil {
		return nil, err
	}
	return buildReturnVO(&ret), nil
}

// ReceiveReturn 退货入库（核心事务）：
//  1. 锁定退货单与各行原出库明细（FOR UPDATE），复查超退；
//  2. 同事务按原出库 unit_cost 回填库存（ApplyInboundTx）并写台账；
//  3. 累计出库行 returned_qty；
//  4. 同事务红字冲销：成本 借 1405/贷 6401，应收 借 6001/贷 1122；
//  5. 退货单置已入库。
func (s *SalesService) ReceiveReturn(ctx context.Context, returnNo, warehouseCode, locationCode string) error {
	warehouse, err := s.warehouseRepo.FindByCode(warehouseCode)
	if err != nil {
		return errors.New(30002, 404, "仓库不存在")
	}
	location, err := s.locationRepo.FindByCode(warehouse.ID, locationCode)
	if err != nil {
		return errors.New(30002, 404, "库位不存在")
	}

	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		ret, err := s.returnRepo.FindByReturnNoForUpdate(tx, returnNo)
		if err != nil {
			return errors.New(70011, 404, "退货单不存在")
		}
		if ret.Status != ReturnStatusPending {
			return errors.New(70004, 409, "退货单已入库或已取消")
		}

		items, err := s.returnRepo.FindItems(ret.ID)
		if err != nil {
			return err
		}
		uid := auth.UserIDFromContext(ctx)
		for _, ri := range items {
			// returned_qty 已在申请时预占，这里只需取原出库成本回填库存。
			locked, err := s.outboundRepo.FindItemForUpdate(tx, ri.OutboundItemID)
			if err != nil {
				return err
			}

			// 按原出库成本回填库存（保持移动平均不被退货价扰动）
			if _, err := s.warehouseService.ApplyInboundTx(ctx, tx, warehouseDto.MoveInput{
				MaterialID:  ri.MaterialID,
				WarehouseID: warehouse.ID,
				LocationID:  location.ID,
				Qty:         ri.Qty,
				UnitCost:    locked.UnitCost,
				BizType:     warehouseService.BizTypeSalesReturn,
				BizNo:       returnNo,
			}); err != nil {
				return err
			}

			// 红字冲销成本：借 库存商品（1405）/ 贷 主营业务成本（6401）
			costAmount := decimalx.Money(locked.UnitCost.Mul(ri.Qty))
			if costAmount.IsPositive() {
				if err := s.finance.PostTx(ctx, tx, financeService.PostingInput{
					BizType:     financeService.BizTypeSalesReturn,
					BizNo:       returnNo,
					Description: fmt.Sprintf("销售退货 %s 冲销成本", returnNo),
					CreatedBy:   uid,
					Lines: []financeService.PostingLine{
						{AccountCode: financeService.AccountFinishedGoods, AccountName: "库存商品", Debit: costAmount},
						{AccountCode: financeService.AccountMainCost, AccountName: "主营业务成本", Credit: costAmount},
					},
				}); err != nil {
					return err
				}
			}
			// 红字冲销应收：借 主营业务收入（6001）/ 贷 应收账款（1122）
			if ri.Amount.IsPositive() {
				if err := s.finance.PostTx(ctx, tx, financeService.PostingInput{
					BizType:     financeService.BizTypeSalesReturn,
					BizNo:       returnNo,
					Description: fmt.Sprintf("销售退货 %s 冲减应收", returnNo),
					CreatedBy:   uid,
					Lines: []financeService.PostingLine{
						{AccountCode: financeService.AccountMainRevenue, AccountName: "主营业务收入", Debit: ri.Amount},
						{AccountCode: financeService.AccountReceivable, AccountName: "应收账款", Credit: ri.Amount},
					},
				}); err != nil {
					return err
				}
			}
		}
		return s.returnRepo.UpdateStatus(tx, ret.ID, ReturnStatusReceived)
	})
}

// GetReturn 退货单详情。
func (s *SalesService) GetReturn(ctx context.Context, returnNo string) (*dto.SalesReturnVO, error) {
	ret, err := s.returnRepo.FindByReturnNo(returnNo)
	if err != nil {
		return nil, errors.New(70011, 404, "退货单不存在")
	}
	return buildReturnVO(ret), nil
}

func buildReturnVO(ret *model.SalSalesReturn) *dto.SalesReturnVO {
	statusDesc := ""
	switch ret.Status {
	case ReturnStatusPending:
		statusDesc = "待入库"
	case ReturnStatusReceived:
		statusDesc = "已入库"
	case ReturnStatusCanceled:
		statusDesc = "已取消"
	}
	return &dto.SalesReturnVO{
		ID:          ret.ID,
		ReturnNo:    ret.ReturnNo,
		OutboundID:  ret.OutboundID,
		CustomerID:  ret.CustomerID,
		Status:      ret.Status,
		StatusDesc:  statusDesc,
		TotalAmount: ret.TotalAmount,
		Reason:      ret.Reason,
		CreatedAt:   ret.CreatedAt.Format(time.RFC3339),
	}
}
