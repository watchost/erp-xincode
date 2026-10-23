// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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
	"erp-system/internal/production/dto"
	"erp-system/internal/production/model"
	"erp-system/internal/production/repository"
	warehouseDto "erp-system/internal/warehouse/dto"
	warehouseService "erp-system/internal/warehouse/service"
)

// 状态码与库表 DEFAULT 统一（迁移 0010）：10 创建 → 20 已下达 → 30 生产中 → 40 已完工；50 已关闭。
const (
	WorkOrderStatusDraft      = 10
	WorkOrderStatusReleased   = 20
	WorkOrderStatusInProgress = 30
	WorkOrderStatusCompleted  = 40
	WorkOrderStatusClosed     = 50
)

// DataScopeResolver 由 IAM service 满足（窄接口，避免反向依赖 iam 包）。
type DataScopeResolver interface {
	ResolveDataScope(ctx context.Context, ownerCol, deptCol string) (datascope.Params, error)
}

type ProductionService struct {
	txManager        *db.TxManager
	workOrderRepo    repository.WorkOrderRepository
	receiptRepo      repository.ProductionReceiptRepository
	bomRepo          repository.BomRepository
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
func (s *ProductionService) SetDataScopeResolver(r DataScopeResolver) {
	s.scopeResolver = r
}

func NewProductionService(
	txManager *db.TxManager,
	workOrderRepo repository.WorkOrderRepository,
	receiptRepo repository.ProductionReceiptRepository,
	bomRepo repository.BomRepository,
	materialRepo mdmRepo.MaterialRepository,
	warehouseRepo mdmRepo.WarehouseRepository,
	locationRepo mdmRepo.LocationRepository,
	warehouseService *warehouseService.WarehouseService,
	finance financeService.Poster,
	numbers *bizno.Generator,
	idempGuard interface {
		Acquire(ctx context.Context, scope, key string) error
	},
) *ProductionService {
	return &ProductionService{
		txManager:        txManager,
		workOrderRepo:    workOrderRepo,
		receiptRepo:      receiptRepo,
		bomRepo:          bomRepo,
		materialRepo:     materialRepo,
		warehouseRepo:    warehouseRepo,
		locationRepo:     locationRepo,
		warehouseService: warehouseService,
		finance:          finance,
		numbers:          numbers,
		idemp:            idempGuard,
	}
}

func (s *ProductionService) acquireIdemp(ctx context.Context, scope, key string) error {
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

func (s *ProductionService) CreateWorkOrder(ctx context.Context, req dto.CreateWorkOrderReq) (*dto.WorkOrderVO, error) {
	material, err := s.materialRepo.FindByID(req.ProductID)
	if err != nil {
		return nil, errors.New(50002, 404, "产品不存在")
	}

	bom, err := s.bomRepo.FindActiveByProductID(req.ProductID)
	if err != nil {
		return nil, errors.New(50003, 404, "未找到产品的BOM")
	}

	bomItems, err := s.bomRepo.FindItems(bom.ID)
	if err != nil {
		return nil, err
	}

	workOrderNo := s.numbers.Next(ctx, "WO")
	planQty := decimalx.Qty(req.PlanQty) // 计划数量 NUMERIC(18,3)

	var workOrder model.ProWorkOrder
	workOrderMaterials := make([]model.ProWorkOrderMaterial, 0, len(bomItems))

	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		workOrder = model.ProWorkOrder{
			WorkOrderNo: workOrderNo,
			ProductID:   req.ProductID,
			PlanQty:     planQty,
			ProducedQty: decimal.Zero,
			Status:      WorkOrderStatusDraft,
			CreatedBy:   auth.UserIDFromContext(ctx),
			DeptID:      auth.DeptIDFromContext(ctx),
		}
		if req.PlanStartAt != "" {
			if t, perr := time.Parse(time.RFC3339, req.PlanStartAt); perr == nil {
				workOrder.PlanStartAt = t
			}
		}
		if req.PlanEndAt != "" {
			if t, perr := time.Parse(time.RFC3339, req.PlanEndAt); perr == nil {
				workOrder.PlanEndAt = t
			}
		}
		if err := s.workOrderRepo.Create(tx, &workOrder); err != nil {
			return err
		}

		for _, item := range bomItems {
			// 物料需求 = BOM 单位用量 × 工单计划量 × (1 + 损耗率)，
			// 结果按工单物料数量标度 NUMERIC(18,3) 归整。
			need := decimalx.Qty(item.Qty.Mul(planQty).Mul(decimal.NewFromInt(1).Add(item.ScrapRate)))
			workOrderMaterials = append(workOrderMaterials, model.ProWorkOrderMaterial{
				WorkOrderID: workOrder.ID,
				MaterialID:  item.MaterialID,
				PlanQty:     need,
				IssuedQty:   decimal.Zero,
				Unit:        item.Unit,
			})
		}

		return s.workOrderRepo.CreateMaterials(tx, workOrderMaterials)
	})
	if err != nil {
		return nil, err
	}

	return s.buildWorkOrderVO(&workOrder, workOrderMaterials, material), nil
}

func (s *ProductionService) ReleaseWorkOrder(ctx context.Context, workOrderNo string) error {
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		order, err := s.workOrderRepo.FindByWorkOrderNoForUpdate(tx, workOrderNo)
		if err != nil {
			return errors.New(50001, 404, "工单不存在")
		}
		if order.Status != WorkOrderStatusDraft {
			return errors.New(50004, 409, "工单状态不允许下达")
		}
		// 齐套检查：下达前校验各物料可用库存（跨仓合计）是否够领料，缺料则整单拒绝。
		materials, err := s.workOrderRepo.FindMaterials(order.ID)
		if err != nil {
			return err
		}
		var shortages []string
		for _, m := range materials {
			need := m.PlanQty.Sub(m.IssuedQty)
			if !need.IsPositive() {
				continue
			}
			avail, err := s.warehouseService.AvailableQtyByMaterial(ctx, m.MaterialID)
			if err != nil {
				return err
			}
			if avail.LessThan(need) {
				code := strconv.FormatInt(m.MaterialID, 10)
				if mat, merr := s.materialRepo.FindByID(m.MaterialID); merr == nil && mat != nil {
					code = mat.MaterialCode
				}
				shortages = append(shortages, fmt.Sprintf("%s(需 %s / 可用 %s)", code, need.String(), avail.String()))
			}
		}
		if len(shortages) > 0 {
			return errors.New(50005, 409, "物料不齐套，缺料: "+strings.Join(shortages, "、"))
		}
		return s.workOrderRepo.UpdateWorkOrderStatus(tx, order.ID, WorkOrderStatusReleased, map[string]interface{}{
			"actual_start_at": time.Now(),
		})
	})
}

// CloseWorkOrder 关闭工单。
//   - 已完工(40)：正常归档关闭；
//   - 已下达(20)/生产中(30)：仅在零领料、零入库时允许中止关闭（库存/成本未发生）；
//   - 草稿(10)/已关闭(50)：不允许。
func (s *ProductionService) CloseWorkOrder(ctx context.Context, workOrderNo string) error {
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		order, err := s.workOrderRepo.FindByWorkOrderNoForUpdate(tx, workOrderNo)
		if err != nil {
			return errors.New(50001, 404, "工单不存在")
		}

		switch order.Status {
		case WorkOrderStatusCompleted:
			// 正常关闭
		case WorkOrderStatusReleased, WorkOrderStatusInProgress:
			materials, err := s.workOrderRepo.FindMaterials(order.ID)
			if err != nil {
				return err
			}
			for _, m := range materials {
				if m.IssuedQty.IsPositive() {
					return errors.New(50006, 409, "工单已有领料记录，不能关闭")
				}
			}
			if order.ProducedQty.IsPositive() {
				return errors.New(50006, 409, "工单已有入库记录，不能关闭")
			}
		default:
			return errors.New(50004, 409, "当前工单状态不允许关闭")
		}

		return s.workOrderRepo.UpdateWorkOrderStatus(tx, order.ID, WorkOrderStatusClosed, map[string]interface{}{
			"actual_end_at": time.Now(),
		})
	})
}

func (s *ProductionService) ListWorkOrders(ctx context.Context, req dto.WorkOrderListReq) ([]*dto.WorkOrderVO, int64, error) {
	var scopes []func(*gorm.DB) *gorm.DB
	if s.scopeResolver != nil {
		params, err := s.scopeResolver.ResolveDataScope(ctx, "created_by", "dept_id")
		if err != nil {
			return nil, 0, err
		}
		scopes = append(scopes, datascope.Apply(params))
	}
	list, total, err := s.workOrderRepo.List(req.WorkOrderNo, req.ProductID, req.Status, req.Page, req.PageSize, scopes...)
	if err != nil {
		return nil, 0, err
	}

	vos := make([]*dto.WorkOrderVO, 0, len(list))
	for i := range list {
		order := list[i]
		material, _ := s.materialRepo.FindByID(order.ProductID)
		materials, _ := s.workOrderRepo.FindMaterials(order.ID)
		vos = append(vos, s.buildWorkOrderVO(&order, materials, material))
	}
	return vos, total, nil
}

func (s *ProductionService) buildWorkOrderVO(order *model.ProWorkOrder, materials []model.ProWorkOrderMaterial, material *mdmModel.MdmMaterial) *dto.WorkOrderVO {
	productCode, productName := "", ""
	if material != nil {
		productCode = material.MaterialCode
		productName = material.Name
	}

	materialVOs := make([]dto.WorkOrderMaterialVO, 0, len(materials))
	for _, m := range materials {
		mat, _ := s.materialRepo.FindByID(m.MaterialID)
		materialCode, materialName := "", ""
		if mat != nil {
			materialCode = mat.MaterialCode
			materialName = mat.Name
		}
		materialVOs = append(materialVOs, dto.WorkOrderMaterialVO{
			ID:           m.ID,
			MaterialID:   m.MaterialID,
			MaterialCode: materialCode,
			MaterialName: materialName,
			PlanQty:      m.PlanQty,
			IssuedQty:    m.IssuedQty,
			Unit:         m.Unit,
		})
	}

	statusDesc := ""
	switch order.Status {
	case WorkOrderStatusDraft:
		statusDesc = "草稿"
	case WorkOrderStatusReleased:
		statusDesc = "已下达"
	case WorkOrderStatusInProgress:
		statusDesc = "生产中"
	case WorkOrderStatusCompleted:
		statusDesc = "已完成"
	case WorkOrderStatusClosed:
		statusDesc = "已关闭"
	}

	return &dto.WorkOrderVO{
		ID:          order.ID,
		WorkOrderNo: order.WorkOrderNo,
		ProductID:   order.ProductID,
		ProductCode: productCode,
		ProductName: productName,
		PlanQty:     order.PlanQty,
		ProducedQty: order.ProducedQty,
		Status:      order.Status,
		StatusDesc:  statusDesc,
		PlanStartAt: order.PlanStartAt.Format(time.RFC3339),
		PlanEndAt:   order.PlanEndAt.Format(time.RFC3339),
		CreatedBy:   order.CreatedBy,
		CreatedAt:   order.CreatedAt.Format(time.RFC3339),
		Materials:   materialVOs,
	}
}

// MaterialIssueScan 生产领料扫码。
// 修复 P0：原来库存扣减（warehouse.Outbound 独立事务）与工单 issued_qty 更新
// 拆成两个事务，且 issued_qty 在应用层校验、并发下会超领。现在合并到同一事务，
// 对工单和工单物料行加 FOR UPDATE 锁，在锁内校验，再在同事务内扣减库存。
func (s *ProductionService) MaterialIssueScan(ctx context.Context, req dto.MaterialIssueScanReq) (*dto.MaterialIssueScanRes, error) {
	if err := s.acquireIdemp(ctx, "production-issue", req.IdempotencyKey); err != nil {
		return nil, err
	}

	material, err := s.materialRepo.FindByCode(req.MaterialCode)
	if err != nil {
		return nil, errors.New(50002, 404, "物料不存在")
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
	var afterQty decimal.Decimal
	var remaining decimal.Decimal

	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		order, err := s.workOrderRepo.FindByWorkOrderNoForUpdate(tx, req.WorkOrderNo)
		if err != nil {
			return errors.New(50001, 404, "工单不存在")
		}
		if order.Status != WorkOrderStatusReleased && order.Status != WorkOrderStatusInProgress {
			return errors.New(50004, 409, "工单未下达，无法领料")
		}

		materials, err := s.workOrderRepo.FindMaterials(order.ID)
		if err != nil {
			return err
		}
		var matched *model.ProWorkOrderMaterial
		for i := range materials {
			if materials[i].MaterialID == material.ID {
				matched = &materials[i]
				break
			}
		}
		if matched == nil {
			return errors.New(50005, 400, "物料不在工单物料清单中")
		}

		// 锁定工单物料行，在锁内校验并更新，杜绝并发超领
		locked, err := s.workOrderRepo.FindMaterialForUpdate(tx, matched.ID)
		if err != nil {
			return err
		}
		reqQty := decimalx.Qty(req.Qty)
		if locked.IssuedQty.Add(reqQty).Cmp(locked.PlanQty) > 0 {
			return errors.New(50006, 400, "领料数量超出计划数量")
		}

		// 同一事务内扣减库存（内部对库存行再加 FOR UPDATE）
		moveRes, err := s.warehouseService.ApplyOutboundTx(ctx, tx, warehouseDto.MoveInput{
			MaterialID:  material.ID,
			WarehouseID: warehouse.ID,
			LocationID:  location.ID,
			Qty:         reqQty,
			BizType:     warehouseService.BizTypeProductionOutbound,
			BizNo:       outboundNo,
		})
		if err != nil {
			return err
		}
		afterQty = moveRes.AfterQty
		remaining = locked.PlanQty.Sub(locked.IssuedQty).Sub(reqQty)

		if err := s.workOrderRepo.UpdateMaterialIssuedQty(tx, locked.ID, reqQty); err != nil {
			return err
		}

		// 同事务过账：借 生产成本（5001）/ 贷 原材料（1403）。
		// 金额按本次出库结转的移动加权平均成本（moveRes.CostAmount），
		// 与库存台账 cost_amount 口径一致；失败随领料事务一并回滚。
		if err := s.finance.PostTx(ctx, tx, financeService.PostingInput{
			BizType:     financeService.BizTypeProductionIssue,
			BizNo:       outboundNo,
			Description: fmt.Sprintf("生产领料 %s（工单 %s）", outboundNo, req.WorkOrderNo),
			Lines: []financeService.PostingLine{
				{AccountCode: financeService.AccountProductionCost, AccountName: "生产成本", Debit: moveRes.CostAmount},
				{AccountCode: financeService.AccountRawMaterials, AccountName: "原材料", Credit: moveRes.CostAmount},
			},
		}); err != nil {
			return err
		}

		// 累计工单领料成本池（完工入库按比例从此池结转，P1.5 S1）。
		if err := s.workOrderRepo.AddOrderCosts(tx, order.ID, moveRes.CostAmount, decimal.Zero); err != nil {
			return err
		}

		if order.Status == WorkOrderStatusReleased {
			return s.workOrderRepo.UpdateWorkOrderStatus(tx, order.ID, WorkOrderStatusInProgress, nil)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &dto.MaterialIssueScanRes{
		OutboundNo: outboundNo,
		Matched:    true,
		DiffQty:    remaining,
		AfterQty:   afterQty,
	}, nil
}

// ProductionReceipt 生产入库/完工报工，与 MaterialIssueScan 对称，全程同一事务：
//  1. 锁定工单（FOR UPDATE），状态须为已下达/生产中，禁止超出工单计划数量入库；
//  2. 入库成本不接受请求传入（P1.5 S1）：从工单成本池（累计领料−累计已结转，
//     即 5001 借方余额）按本次入库量占剩余计划量比例结转；完工收尾时全额
//     结转剩余成本池，保证工单完工后 5001 余额轧平为零；
//  3. 成品（工单 ProductID）同事务 ApplyInboundTx 入指定仓/库位（移动加权计价）；
//  4. 写生产入库单 prod_production_receipt；累计 received_cost；
//  5. 同事务过账：借 库存商品（1405）/ 贷 生产成本（5001），金额与库存台账
//     cost_amount 口径一致；
//  6. produced_qty 累加；达计划数量则置已完成并记 actual_end_at，否则已下达→生产中。
func (s *ProductionService) ProductionReceipt(ctx context.Context, req dto.ProductionReceiptReq) (*dto.ProductionReceiptRes, error) {
	if err := s.acquireIdemp(ctx, "production-receipt", req.IdempotencyKey); err != nil {
		return nil, err
	}

	warehouse, err := s.warehouseRepo.FindByCode(req.WarehouseCode)
	if err != nil {
		return nil, errors.New(30002, 404, "仓库不存在")
	}
	location, err := s.locationRepo.FindByCode(warehouse.ID, req.LocationCode)
	if err != nil {
		return nil, errors.New(30002, 404, "库位不存在")
	}

	receiptNo := s.numbers.Next(ctx, "PR")
	var afterQty decimal.Decimal
	var finalStatus int
	completed := false

	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		order, err := s.workOrderRepo.FindByWorkOrderNoForUpdate(tx, req.WorkOrderNo)
		if err != nil {
			return errors.New(50001, 404, "工单不存在")
		}
		if order.Status != WorkOrderStatusReleased && order.Status != WorkOrderStatusInProgress {
			return errors.New(50004, 409, "工单未下达或已完工，无法入库")
		}
		reqQty := decimalx.Qty(req.Qty)
		if !reqQty.IsPositive() {
			return errors.New(50006, 400, "入库数量低于最小精度")
		}
		if order.ProducedQty.Add(reqQty).Cmp(order.PlanQty) > 0 {
			return errors.New(50006, 400, "入库数量超出工单计划数量")
		}

		// 成本来源（P1.5 S1）：工单成本池 = 累计领料成本 − 累计已结转成本。
		// 分批入库按 本次量/剩余计划量 比例结转；本次达计划量（完工）则全额
		// 结转剩余池，吸收历次舍入尾差，保证 5001 在完工时点轧平。
		pool := order.IssuedCost.Sub(order.ReceivedCost)
		if !pool.IsPositive() {
			return errors.New(50007, 400, "工单尚无已领料成本，请先领料再入库")
		}
		remainingPlan := order.PlanQty.Sub(order.ProducedQty)
		completing := order.ProducedQty.Add(reqQty).Cmp(order.PlanQty) >= 0
		var amount decimal.Decimal
		if completing {
			amount = decimalx.Money(pool)
		} else {
			amount = decimalx.Money(pool.Mul(reqQty).Div(remainingPlan))
		}
		if !amount.IsPositive() {
			return errors.New(50007, 400, "本次入库结转成本为零，请合并到完工批次入库")
		}
		reqCost := decimalx.Cost(amount.Div(reqQty)) // 单位成本 NUMERIC(18,4)

		// 成品入库：成品即工单 ProductID；同事务写库存与台账（内部再锁库存行）。
		moveRes, err := s.warehouseService.ApplyInboundTx(ctx, tx, warehouseDto.MoveInput{
			MaterialID:  order.ProductID,
			WarehouseID: warehouse.ID,
			LocationID:  location.ID,
			Qty:         reqQty,
			UnitCost:    reqCost,
			BizType:     warehouseService.BizTypeProductionInbound,
			BizNo:       receiptNo,
		})
		if err != nil {
			return err
		}
		afterQty = moveRes.AfterQty

		receipt := model.ProProductionReceipt{
			ReceiptNo:   receiptNo,
			WorkOrderID: order.ID,
			MaterialID:  order.ProductID,
			WarehouseID: warehouse.ID,
			Qty:         reqQty,
			UnitCost:    reqCost,
			Status:      1,
			CreatedBy:   auth.UserIDFromContext(ctx),
		}
		if err := s.receiptRepo.Create(tx, &receipt); err != nil {
			return err
		}

		// 累计已结转成本（与台账/过账同一金额来源 moveRes.CostAmount）。
		if err := s.workOrderRepo.AddOrderCosts(tx, order.ID, decimal.Zero, moveRes.CostAmount); err != nil {
			return err
		}

		// 同事务过账：借 库存商品（1405）/ 贷 生产成本（5001），结转完工成本。
		if err := s.finance.PostTx(ctx, tx, financeService.PostingInput{
			BizType:     financeService.BizTypeProductionReceipt,
			BizNo:       receiptNo,
			Description: fmt.Sprintf("生产入库 %s（工单 %s）", receiptNo, req.WorkOrderNo),
			Lines: []financeService.PostingLine{
				{AccountCode: financeService.AccountFinishedGoods, AccountName: "库存商品", Debit: moveRes.CostAmount},
				{AccountCode: financeService.AccountProductionCost, AccountName: "生产成本", Credit: moveRes.CostAmount},
			},
		}); err != nil {
			return err
		}

		// 工单进度：累加 produced_qty；达计划数量则完工（记 actual_end_at），否则推进到生产中。
		fields := map[string]interface{}{
			"produced_qty": gorm.Expr("produced_qty + ?", reqQty),
		}
		if completing {
			fields["actual_end_at"] = time.Now()
			finalStatus = WorkOrderStatusCompleted
			completed = true
		} else {
			finalStatus = WorkOrderStatusInProgress
		}
		return s.workOrderRepo.UpdateWorkOrderStatus(tx, order.ID, finalStatus, fields)
	})
	if err != nil {
		return nil, err
	}

	return &dto.ProductionReceiptRes{
		ReceiptNo:       receiptNo,
		Matched:         true,
		AfterQty:        afterQty,
		WorkOrderStatus: finalStatus,
		Completed:       completed,
	}, nil
}

func (s *ProductionService) CreateBom(ctx context.Context, req dto.CreateBomReq) (*dto.BomVO, error) {
	product, err := s.materialRepo.FindByID(req.ProductID)
	if err != nil {
		return nil, errors.New(50002, 404, "产品不存在")
	}

	bomVersion := req.BomVersion
	if bomVersion == "" {
		bomVersion = "V1.0"
	}

	bomItems := make([]model.ProBomItem, 0, len(req.Items))
	for _, item := range req.Items {
		bomItems = append(bomItems, model.ProBomItem{
			MaterialID: item.MaterialID,
			Qty:        item.Qty,
			Unit:       item.Unit,
			ScrapRate:  item.ScrapRate,
			Sequence:   item.Sequence,
		})
	}

	var bom model.ProBom
	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		if err := s.bomRepo.DeactivateByProductID(tx, req.ProductID); err != nil {
			return err
		}

		bom = model.ProBom{
			ProductID:  req.ProductID,
			BomVersion: bomVersion,
			IsActive:   true,
			CreatedBy:  auth.UserIDFromContext(ctx),
		}
		if req.EffectiveStart != "" {
			if t, perr := time.Parse(time.RFC3339, req.EffectiveStart); perr == nil {
				bom.EffectiveStart = t
			}
		}
		if bom.EffectiveStart.IsZero() {
			bom.EffectiveStart = time.Now()
		}
		if err := s.bomRepo.Create(tx, &bom); err != nil {
			return err
		}
		for i := range bomItems {
			bomItems[i].BomID = bom.ID
		}
		return s.bomRepo.CreateItems(tx, bomItems)
	})
	if err != nil {
		return nil, err
	}
	return s.buildBomVO(&bom, bomItems, product), nil
}

func (s *ProductionService) buildBomVO(bom *model.ProBom, items []model.ProBomItem, product *mdmModel.MdmMaterial) *dto.BomVO {
	productCode, productName := "", ""
	if product != nil {
		productCode = product.MaterialCode
		productName = product.Name
	}
	itemVOs := make([]dto.BomItemVO, 0, len(items))
	for _, item := range items {
		m, _ := s.materialRepo.FindByID(item.MaterialID)
		materialCode, materialName := "", ""
		if m != nil {
			materialCode = m.MaterialCode
			materialName = m.Name
		}
		itemVOs = append(itemVOs, dto.BomItemVO{
			ID:           item.ID,
			MaterialID:   item.MaterialID,
			MaterialCode: materialCode,
			MaterialName: materialName,
			Qty:          item.Qty,
			Unit:         item.Unit,
			ScrapRate:    item.ScrapRate,
			Sequence:     item.Sequence,
		})
	}
	return &dto.BomVO{
		ID:             bom.ID,
		ProductID:      bom.ProductID,
		ProductCode:    productCode,
		ProductName:    productName,
		BomVersion:     bom.BomVersion,
		IsActive:       bom.IsActive,
		EffectiveStart: bom.EffectiveStart.Format(time.RFC3339),
		EffectiveEnd:   bom.EffectiveEnd.Format(time.RFC3339),
		CreatedAt:      bom.CreatedAt.Format(time.RFC3339),
		Items:          itemVOs,
	}
}
