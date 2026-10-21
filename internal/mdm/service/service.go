// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"encoding/json"

	"erp-system/internal/mdm/dto"
	"erp-system/internal/mdm/model"
	"erp-system/internal/mdm/repository"
	"erp-system/internal/pkg/errors"
	"gorm.io/gorm"
)

// InventoryChecker 判断主数据是否仍被库存引用，用于删除前的引用守卫。
// 由 warehouse 模块在装配层以适配器注入，避免 mdm 反向依赖 warehouse。
type InventoryChecker interface {
	MaterialOnHand(ctx context.Context, materialID int64) (bool, error)
	WarehouseInUse(ctx context.Context, warehouseID int64) (bool, error)
	LocationInUse(ctx context.Context, locationID int64) (bool, error)
}

type MDMService struct {
	materialRepo  repository.MaterialRepository
	supplierRepo  repository.SupplierRepository
	warehouseRepo repository.WarehouseRepository
	locationRepo  repository.LocationRepository
	categoryRepo  repository.CategoryRepository
	customerRepo  repository.CustomerRepository
	inventory     InventoryChecker
}

func NewMDMService(
	materialRepo repository.MaterialRepository,
	supplierRepo repository.SupplierRepository,
	warehouseRepo repository.WarehouseRepository,
	locationRepo repository.LocationRepository,
	categoryRepo repository.CategoryRepository,
	customerRepo repository.CustomerRepository,
	inventory InventoryChecker,
) *MDMService {
	return &MDMService{
		materialRepo:  materialRepo,
		supplierRepo:  supplierRepo,
		warehouseRepo: warehouseRepo,
		locationRepo:  locationRepo,
		categoryRepo:  categoryRepo,
		customerRepo:  customerRepo,
		inventory:     inventory,
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
			ID:             m.ID,
			MaterialCode:   m.MaterialCode,
			Name:           m.Name,
			Spec:           m.Spec,
			CategoryID:     m.CategoryID,
			Unit:           m.Unit,
			PurchaseUnit:   m.PurchaseUnit,
			StockUnit:      m.StockUnit,
			ConversionRate: m.ConversionRate,
			CostMethod:     m.CostMethod,
			Attributes:     attrs,
			Status:         m.Status,
			MinStock:       m.MinStock,
			MaxStock:       m.MaxStock,
			BatchManaged:   m.BatchManaged,
			SerialManaged:  m.SerialManaged,
			ShelfLifeDays:  m.ShelfLifeDays,
			CreatedAt:      m.CreatedAt.Format("2006-01-02 15:04:05"),
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
		ID:             m.ID,
		MaterialCode:   m.MaterialCode,
		Name:           m.Name,
		Spec:           m.Spec,
		CategoryID:     m.CategoryID,
		Unit:           m.Unit,
		PurchaseUnit:   m.PurchaseUnit,
		StockUnit:      m.StockUnit,
		ConversionRate: m.ConversionRate,
		CostMethod:     m.CostMethod,
		Attributes:     attrs,
		Status:         m.Status,
		MinStock:       m.MinStock,
		MaxStock:       m.MaxStock,
		BatchManaged:   m.BatchManaged,
		SerialManaged:  m.SerialManaged,
		ShelfLifeDays:  m.ShelfLifeDays,
		CreatedAt:      m.CreatedAt.Format("2006-01-02 15:04:05"),
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
	m, err := s.materialRepo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New(10100, 404, "物料不存在")
		}
		return err
	}
	if s.inventory != nil {
		onHand, err := s.inventory.MaterialOnHand(ctx, m.ID)
		if err != nil {
			return err
		}
		if onHand {
			return errors.New(10006, 409, "物料存在库存余额，不能删除")
		}
	}
	return s.materialRepo.Delete(id)
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
	if _, err := s.supplierRepo.FindByID(id); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New(10100, 404, "供应商不存在")
		}
		return err
	}
	return s.supplierRepo.Delete(id)
}

func (s *MDMService) CreateWarehouse(ctx context.Context, w *model.MdmWarehouse) error {
	if w.Code == "" || w.Name == "" {
		return errors.New(10005, 400, "仓库编码和名称不能为空")
	}
	existing, _ := s.warehouseRepo.FindByCode(w.Code)
	if existing != nil {
		return errors.New(10005, 400, "仓库编码已存在")
	}
	return s.warehouseRepo.Create(w)
}

func (s *MDMService) UpdateWarehouse(ctx context.Context, w *model.MdmWarehouse) error {
	if w.ID == 0 {
		return errors.New(10005, 400, "仓库 ID 不能为空")
	}
	if _, err := s.warehouseRepo.FindByID(w.ID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New(10100, 404, "仓库不存在")
		}
		return err
	}
	return s.warehouseRepo.Update(w)
}

func (s *MDMService) DeleteWarehouse(ctx context.Context, id int64) error {
	if _, err := s.warehouseRepo.FindByID(id); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New(10100, 404, "仓库不存在")
		}
		return err
	}
	if n, err := s.locationRepo.CountByWarehouse(id); err != nil {
		return err
	} else if n > 0 {
		return errors.New(10006, 409, "仓库下仍有库位，不能删除")
	}
	if s.inventory != nil {
		inUse, err := s.inventory.WarehouseInUse(ctx, id)
		if err != nil {
			return err
		}
		if inUse {
			return errors.New(10006, 409, "仓库存在库存，不能删除")
		}
	}
	return s.warehouseRepo.Delete(id)
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
			ID:            l.ID,
			WarehouseID:   l.WarehouseID,
			WarehouseName: warehouseName,
			LocationCode:  l.LocationCode,
			Zone:          l.Zone,
			CreatedAt:     l.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return vos, nil
}

func (s *MDMService) GetLocation(ctx context.Context, id int64) (*dto.LocationVO, error) {
	l, err := s.locationRepo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(10100, 404, "库位不存在")
		}
		return nil, err
	}
	return s.buildLocationVO(l)
}

func (s *MDMService) CreateLocation(ctx context.Context, l *model.MdmLocation) error {
	if l.WarehouseID == 0 || l.LocationCode == "" {
		return errors.New(10005, 400, "所属仓库和库位编码不能为空")
	}
	if _, err := s.warehouseRepo.FindByID(l.WarehouseID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New(10100, 404, "所属仓库不存在")
		}
		return err
	}
	existing, _ := s.locationRepo.FindByCode(l.WarehouseID, l.LocationCode)
	if existing != nil {
		return errors.New(10005, 400, "该仓库下库位编码已存在")
	}
	return s.locationRepo.Create(l)
}

func (s *MDMService) UpdateLocation(ctx context.Context, l *model.MdmLocation) error {
	if l.ID == 0 {
		return errors.New(10005, 400, "库位 ID 不能为空")
	}
	if _, err := s.locationRepo.FindByID(l.ID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New(10100, 404, "库位不存在")
		}
		return err
	}
	return s.locationRepo.Update(l)
}

func (s *MDMService) DeleteLocation(ctx context.Context, id int64) error {
	if _, err := s.locationRepo.FindByID(id); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New(10100, 404, "库位不存在")
		}
		return err
	}
	if s.inventory != nil {
		inUse, err := s.inventory.LocationInUse(ctx, id)
		if err != nil {
			return err
		}
		if inUse {
			return errors.New(10006, 409, "库位存在库存，不能删除")
		}
	}
	return s.locationRepo.Delete(id)
}

func (s *MDMService) buildLocationVO(l *model.MdmLocation) (*dto.LocationVO, error) {
	warehouseName := ""
	if w, err := s.warehouseRepo.FindByID(l.WarehouseID); err == nil && w != nil {
		warehouseName = w.Name
	}
	return &dto.LocationVO{
		ID:            l.ID,
		WarehouseID:   l.WarehouseID,
		WarehouseName: warehouseName,
		LocationCode:  l.LocationCode,
		Zone:          l.Zone,
		CreatedAt:     l.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}
