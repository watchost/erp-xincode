// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"
	"erp-system/internal/mdm/dto"
	"erp-system/internal/mdm/model"
	"erp-system/internal/mdm/repository"
	"erp-system/internal/pkg/errors"
)

type MDMService struct {
	materialRepo   repository.MaterialRepository
	supplierRepo   repository.SupplierRepository
	warehouseRepo  repository.WarehouseRepository
	locationRepo   repository.LocationRepository
}

func NewMDMService(
	materialRepo repository.MaterialRepository,
	supplierRepo repository.SupplierRepository,
	warehouseRepo repository.WarehouseRepository,
	locationRepo repository.LocationRepository,
) *MDMService {
	return &MDMService{
		materialRepo:  materialRepo,
		supplierRepo:  supplierRepo,
		warehouseRepo: warehouseRepo,
		locationRepo:  locationRepo,
	}
}

func (s *MDMService) ListMaterials(ctx context.Context, req dto.MaterialListReq) ([]dto.MaterialVO, int64, error) {
	materials, total, err := s.materialRepo.List(req.MaterialCode, req.Name, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}

	var vos []dto.MaterialVO
	for _, m := range materials {
		var attrs map[string]any
		if m.Attributes != nil {
			json.Unmarshal(m.Attributes, &attrs)
		}
		vos = append(vos, dto.MaterialVO{
			ID:            m.ID,
			MaterialCode:  m.MaterialCode,
			Name:          m.Name,
			Spec:          m.Spec,
			CategoryID:    m.CategoryID,
			Unit:          m.Unit,
			CostMethod:    m.CostMethod,
			Attributes:    attrs,
			Status:        m.Status,
			CreatedAt:     m.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return vos, total, nil
}

func (s *MDMService) GetMaterial(ctx context.Context, id int64) (*dto.MaterialVO, error) {
	m, err := s.materialRepo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(10100, 404, "物料不存在")
		}
		return nil, err
	}

	var attrs map[string]any
	if m.Attributes != nil {
		json.Unmarshal(m.Attributes, &attrs)
	}

	return &dto.MaterialVO{
		ID:            m.ID,
		MaterialCode:  m.MaterialCode,
		Name:          m.Name,
		Spec:          m.Spec,
		CategoryID:    m.CategoryID,
		Unit:          m.Unit,
		CostMethod:    m.CostMethod,
		Attributes:    attrs,
		Status:        m.Status,
		CreatedAt:     m.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

func (s *MDMService) CreateMaterial(ctx context.Context, m *model.MdmMaterial) error {
	existing, _ := s.materialRepo.FindByCode(m.MaterialCode)
	if existing != nil {
		return errors.New(10005, 400, "物料编码已存在")
	}
	return s.materialRepo.Create(m)
}

func (s *MDMService) UpdateMaterial(ctx context.Context, m *model.MdmMaterial) error {
	return s.materialRepo.Update(m)
}

func (s *MDMService) DeleteMaterial(ctx context.Context, id int64) error {
	return nil
}

func (s *MDMService) ListSuppliers(ctx context.Context, req dto.SupplierListReq) ([]dto.SupplierVO, int64, error) {
	suppliers, total, err := s.supplierRepo.List(req.SupplierCode, req.Name, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}

	var vos []dto.SupplierVO
	for _, s := range suppliers {
		var attrs map[string]any
		if s.Attributes != nil {
			json.Unmarshal(s.Attributes, &attrs)
		}
		vos = append(vos, dto.SupplierVO{
			ID:           s.ID,
			SupplierCode: s.SupplierCode,
			Name:         s.Name,
			Contact:      s.Contact,
			Level:        s.Level,
			Attributes:   attrs,
			CreatedAt:    s.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return vos, total, nil
}

func (s *MDMService) GetSupplier(ctx context.Context, id int64) (*dto.SupplierVO, error) {
	sup, err := s.supplierRepo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(10100, 404, "供应商不存在")
		}
		return nil, err
	}

	var attrs map[string]any
	if sup.Attributes != nil {
		json.Unmarshal(sup.Attributes, &attrs)
	}

	return &dto.SupplierVO{
		ID:           sup.ID,
		SupplierCode: sup.SupplierCode,
		Name:         sup.Name,
		Contact:      sup.Contact,
		Level:        sup.Level,
		Attributes:   attrs,
		CreatedAt:    sup.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

func (s *MDMService) CreateSupplier(ctx context.Context, sup *model.MdmSupplier) error {
	existing, _ := s.supplierRepo.FindByCode(sup.SupplierCode)
	if existing != nil {
		return errors.New(10005, 400, "供应商编码已存在")
	}
	return s.supplierRepo.Create(sup)
}

func (s *MDMService) UpdateSupplier(ctx context.Context, sup *model.MdmSupplier) error {
	return s.supplierRepo.Update(sup)
}

func (s *MDMService) DeleteSupplier(ctx context.Context, id int64) error {
	return nil
}

func (s *MDMService) CreateWarehouse(ctx context.Context, w *model.MdmWarehouse) error {
	return nil
}

func (s *MDMService) ListWarehouses(ctx context.Context) ([]dto.WarehouseVO, error) {
	warehouses, err := s.warehouseRepo.List()
	if err != nil {
		return nil, err
	}

	var vos []dto.WarehouseVO
	for _, w := range warehouses {
		vos = append(vos, dto.WarehouseVO{
			ID:        w.ID,
			Code:      w.Code,
			Name:      w.Name,
			Type:      w.Type,
			CreatedAt: w.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return vos, nil
}

func (s *MDMService) GetWarehouse(ctx context.Context, id int64) (*dto.WarehouseVO, error) {
	w, err := s.warehouseRepo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(10100, 404, "仓库不存在")
		}
		return nil, err
	}

	return &dto.WarehouseVO{
		ID:        w.ID,
		Code:      w.Code,
		Name:      w.Name,
		Type:      w.Type,
		CreatedAt: w.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

func (s *MDMService) ListLocations(ctx context.Context, warehouseID int64) ([]dto.LocationVO, error) {
	locations, err := s.locationRepo.List(warehouseID)
	if err != nil {
		return nil, err
	}

	var vos []dto.LocationVO
	for _, l := range locations {
		w, _ := s.warehouseRepo.FindByID(l.WarehouseID)
		warehouseName := ""
		if w != nil {
			warehouseName = w.Name
		}
		vos = append(vos, dto.LocationVO{
			ID:           l.ID,
			WarehouseID:  l.WarehouseID,
			WarehouseName: warehouseName,
			LocationCode: l.LocationCode,
			Zone:         l.Zone,
			CreatedAt:    l.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return vos, nil
}

func (s *MDMService) CreateLocation(ctx context.Context, l *model.MdmLocation) error {
	return nil
}
