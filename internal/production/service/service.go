// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	mdmModel "erp-system/internal/mdm/model"
	mdmRepo "erp-system/internal/mdm/repository"
	"erp-system/internal/production/dto"
	"erp-system/internal/production/model"
	"erp-system/internal/production/repository"
	"erp-system/internal/pkg/db"
	"erp-system/internal/pkg/errors"
	warehouseService "erp-system/internal/warehouse/service"
	warehouseDto "erp-system/internal/warehouse/dto"
)

const (
	WorkOrderStatusDraft      = 1
	WorkOrderStatusReleased   = 2
	WorkOrderStatusInProgress = 3
	WorkOrderStatusCompleted  = 4
)

type ProductionService struct {
	txManager           *db.TxManager
	workOrderRepo       repository.WorkOrderRepository
	bomRepo             repository.BomRepository
	materialRepo        mdmRepo.MaterialRepository
	warehouseService    *warehouseService.WarehouseService
}

func NewProductionService(
	txManager *db.TxManager,
	workOrderRepo repository.WorkOrderRepository,
	bomRepo repository.BomRepository,
	materialRepo mdmRepo.MaterialRepository,
	warehouseService *warehouseService.WarehouseService,
) *ProductionService {
	return &ProductionService{
		txManager:        txManager,
		workOrderRepo:    workOrderRepo,
		bomRepo:          bomRepo,
		materialRepo:     materialRepo,
		warehouseService: warehouseService,
	}
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

	workOrderNo := fmt.Sprintf("WO%v", time.Now().Unix())

	var workOrder model.ProWorkOrder
	var workOrderMaterials []model.ProWorkOrderMaterial

	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		workOrder = model.ProWorkOrder{
			WorkOrderNo: workOrderNo,
			ProductID:   req.ProductID,
			PlanQty:     req.PlanQty,
			ProducedQty: 0,
			Status:      WorkOrderStatusDraft,
			CreatedBy:   1,
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
		order, err := s.workOrderRepo.FindByWorkOrderNo(workOrderNo)
		if err != nil {
			return errors.New(50001, 404, "工单不存在")
		}
		if order.Status != WorkOrderStatusDraft {
			return errors.New(50004, 409, "工单状态不允许下达")
		}
		return s.workOrderRepo.UpdateWorkOrderStatus(tx, order.ID, WorkOrderStatusReleased)
	})
}

func (s *ProductionService) ListWorkOrders(ctx context.Context, req dto.WorkOrderListReq) ([]*dto.WorkOrderVO, int64, error) {
	list, total, err := s.workOrderRepo.List(req.WorkOrderNo, req.ProductID, req.Status, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}

	var vos []*dto.WorkOrderVO
	for _, order := range list {
		material, _ := s.materialRepo.FindByID(order.ProductID)
		materials, _ := s.workOrderRepo.FindMaterials(order.ID)
		vos = append(vos, s.buildWorkOrderVO(&order, materials, material))
	}

	return vos, total, nil
}

func (s *ProductionService) buildWorkOrderVO(order *model.ProWorkOrder, materials []model.ProWorkOrderMaterial, material *mdmModel.MdmMaterial) *dto.WorkOrderVO {
	productCode := ""
	productName := ""
	if material != nil {
		productCode = material.MaterialCode
		productName = material.Name
	}

	var materialVOs []dto.WorkOrderMaterialVO
	for _, m := range materials {
		mat, _ := s.materialRepo.FindByID(m.MaterialID)
		materialCode := ""
		materialName := ""
		if mat != nil {
			materialCode = mat.MaterialCode
			materialName = mat.Name
		}
		materialVOs = append(materialVOs, dto.WorkOrderMaterialVO{
			ID:          m.ID,
			MaterialID:  m.MaterialID,
			MaterialCode: materialCode,
			MaterialName: materialName,
			PlanQty:     m.PlanQty,
			IssuedQty:   m.IssuedQty,
			Unit:        m.Unit,
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
		ID:             order.ID,
		WorkOrderNo:    order.WorkOrderNo,
		ProductID:      order.ProductID,
		ProductCode:    productCode,
		ProductName:    productName,
		PlanQty:        order.PlanQty,
		ProducedQty:    order.ProducedQty,
		Status:         order.Status,
		StatusDesc:     statusDesc,
		PlanStartAt:    order.PlanStartAt.Format(time.RFC3339),
		PlanEndAt:      order.PlanEndAt.Format(time.RFC3339),
		CreatedBy:      order.CreatedBy,
		CreatedAt:      order.CreatedAt.Format(time.RFC3339),
		Materials:      materialVOs,
	}
}

func (s *ProductionService) MaterialIssueScan(ctx context.Context, req dto.MaterialIssueScanReq) (*dto.MaterialIssueScanRes, error) {
	order, err := s.workOrderRepo.FindByWorkOrderNo(req.WorkOrderNo)
	if err != nil {
		return nil, errors.New(50001, 404, "工单不存在")
	}
	if order.Status != WorkOrderStatusReleased && order.Status != WorkOrderStatusInProgress {
		return nil, errors.New(50004, 409, "工单未下达，无法领料")
	}

	materials, err := s.workOrderRepo.FindMaterials(order.ID)
	if err != nil {
		return nil, err
	}

	material, err := s.materialRepo.FindByCode(req.MaterialCode)
	if err != nil {
		return nil, errors.New(50002, 404, "物料不存在")
	}

	var matchedMaterial *model.ProWorkOrderMaterial
	for i := range materials {
		if materials[i].MaterialID == material.ID {
			matchedMaterial = &materials[i]
			break
		}
	}
	if matchedMaterial == nil {
		return nil, errors.New(50005, 400, "物料不在工单物料清单中")
	}

	if matchedMaterial.IssuedQty+req.Qty > matchedMaterial.PlanQty {
		return nil, errors.New(50006, 400, "领料数量超出计划数量")
	}

	outboundReq := warehouseDto.OutboundScanReq{
		MaterialCode: req.MaterialCode,
		Qty:          req.Qty,
		OutboundNo:   req.WorkOrderNo,
	}

	warehouseRes, err := s.warehouseService.Outbound(ctx, outboundReq)
	if err != nil {
		return nil, err
	}

	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		if err := s.workOrderRepo.UpdateMaterialIssuedQty(tx, matchedMaterial.ID, req.Qty); err != nil {
			return err
		}

		if order.Status == WorkOrderStatusReleased {
			return s.workOrderRepo.UpdateWorkOrderStatus(tx, order.ID, WorkOrderStatusInProgress)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &dto.MaterialIssueScanRes{
		OutboundNo: warehouseRes.InboundNo,
		Matched:    true,
		DiffQty:    matchedMaterial.PlanQty - matchedMaterial.IssuedQty - req.Qty,
		AfterQty:   warehouseRes.AfterQty,
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

	var bomItems []model.ProBomItem
	for _, item := range req.Items {
		bomItems = append(bomItems, model.ProBomItem{
			BomID:       0,
			MaterialID:  item.MaterialID,
			Qty:         item.Qty,
			Unit:        item.Unit,
			ScrapRate:   item.ScrapRate,
			Sequence:    item.Sequence,
		})
	}

	var bom model.ProBom
	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		if err := s.bomRepo.DeactivateByProductID(tx, req.ProductID); err != nil {
			return err
		}

		bom = model.ProBom{
			ProductID:      req.ProductID,
			BomVersion:     bomVersion,
			IsActive:       true,
			CreatedBy:      1,
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
	productCode := ""
	productName := ""
	if product != nil {
		productCode = product.MaterialCode
		productName = product.Name
	}

	var itemVOs []dto.BomItemVO
	for _, item := range items {
		m, _ := s.materialRepo.FindByID(item.MaterialID)
		materialCode := ""
		materialName := ""
		if m != nil {
			materialCode = m.MaterialCode
			materialName = m.Name
		}
		itemVOs = append(itemVOs, dto.BomItemVO{
			ID:          item.ID,
			MaterialID:  item.MaterialID,
			MaterialCode: materialCode,
			MaterialName: materialName,
			Qty:         item.Qty,
			Unit:        item.Unit,
			ScrapRate:   item.ScrapRate,
			Sequence:    item.Sequence,
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
