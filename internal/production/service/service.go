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
	"erp-system/internal/production/dto"
	"erp-system/internal/production/model"
	"erp-system/internal/production/repository"
	warehouseDto "erp-system/internal/warehouse/dto"
	warehouseService "erp-system/internal/warehouse/service"
)

const (
	WorkOrderStatusDraft      = 1
	WorkOrderStatusReleased   = 2
	WorkOrderStatusInProgress = 3
	WorkOrderStatusCompleted  = 4
)

type ProductionService struct {
	txManager        *db.TxManager
	workOrderRepo    repository.WorkOrderRepository
	bomRepo          repository.BomRepository
	materialRepo     mdmRepo.MaterialRepository
	warehouseRepo    mdmRepo.WarehouseRepository
	locationRepo     mdmRepo.LocationRepository
	warehouseService *warehouseService.WarehouseService
	numbers          *bizno.Generator
	idemp            interface {
		Acquire(ctx context.Context, scope, key string) error
	}
}

func NewProductionService(
	txManager *db.TxManager,
	workOrderRepo repository.WorkOrderRepository,
	bomRepo repository.BomRepository,
	materialRepo mdmRepo.MaterialRepository,
	warehouseRepo mdmRepo.WarehouseRepository,
	locationRepo mdmRepo.LocationRepository,
	warehouseService *warehouseService.WarehouseService,
	numbers *bizno.Generator,
	idempGuard interface {
		Acquire(ctx context.Context, scope, key string) error
	},
) *ProductionService {
	return &ProductionService{
		txManager:        txManager,
		workOrderRepo:    workOrderRepo,
		bomRepo:          bomRepo,
		materialRepo:     materialRepo,
		warehouseRepo:    warehouseRepo,
		locationRepo:     locationRepo,
		warehouseService: warehouseService,
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

	var workOrder model.ProWorkOrder
	workOrderMaterials := make([]model.ProWorkOrderMaterial, 0, len(bomItems))

	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		workOrder = model.ProWorkOrder{
			WorkOrderNo: workOrderNo,
			ProductID:   req.ProductID,
			PlanQty:     req.PlanQty,
			ProducedQty: 0,
			Status:      WorkOrderStatusDraft,
			CreatedBy:   auth.UserIDFromContext(ctx),
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
			planQty := item.Qty * req.PlanQty * (1 + item.ScrapRate)
			workOrderMaterials = append(workOrderMaterials, model.ProWorkOrderMaterial{
				WorkOrderID: workOrder.ID,
				MaterialID:  item.MaterialID,
				PlanQty:     planQty,
				IssuedQty:   0,
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
		// TODO(P1): 齐套检查——遍历 BOM 物料校验可用库存
		return s.workOrderRepo.UpdateWorkOrderStatus(tx, order.ID, WorkOrderStatusReleased, map[string]interface{}{
			"actual_start_at": time.Now(),
		})
	})
}

func (s *ProductionService) ListWorkOrders(ctx context.Context, req dto.WorkOrderListReq) ([]*dto.WorkOrderVO, int64, error) {
	list, total, err := s.workOrderRepo.List(req.WorkOrderNo, req.ProductID, req.Status, req.Page, req.PageSize)
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
	var afterQty float64
	var remaining float64

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
		if locked.IssuedQty+req.Qty > locked.PlanQty {
			return errors.New(50006, 400, "领料数量超出计划数量")
		}

		// 同一事务内扣减库存（内部对库存行再加 FOR UPDATE）
		moveRes, err := s.warehouseService.ApplyOutboundTx(ctx, tx, warehouseDto.MoveInput{
			MaterialID:  material.ID,
			WarehouseID: warehouse.ID,
			LocationID:  location.ID,
			Qty:         req.Qty,
			BizType:     warehouseService.BizTypeProductionOutbound,
			BizNo:       outboundNo,
		})
		if err != nil {
			return err
		}
		afterQty = moveRes.AfterQty
		remaining = locked.PlanQty - locked.IssuedQty - req.Qty

		if err := s.workOrderRepo.UpdateMaterialIssuedQty(tx, locked.ID, req.Qty); err != nil {
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
			ProductID: req.ProductID,
			BomVersion: bomVersion,
			IsActive:  true,
			CreatedBy: auth.UserIDFromContext(ctx),
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
