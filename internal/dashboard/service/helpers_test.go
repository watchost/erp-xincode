// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"testing"

	"gorm.io/gorm"

	mdmModel "erp-system/internal/mdm/model"
	"erp-system/internal/testsupport"
	warehouseDto "erp-system/internal/warehouse/dto"
	warehouseService "erp-system/internal/warehouse/service"
)

// createMaterialWithMinStock 建一个带可空安全库存的物料（min_stock NULL 档由夹具物料覆盖）。
func createMaterialWithMinStock(t *testing.T, gdb *gorm.DB, code, minStock string) *mdmModel.MdmMaterial {
	t.Helper()
	ms := testsupport.D(minStock)
	mat := &mdmModel.MdmMaterial{
		MaterialCode: code,
		Name:         "仪表盘测试物料-" + code,
		Unit:         "个",
		CostMethod:   1,
		Status:       1,
		MinStock:     &ms,
	}
	if err := gdb.Create(mat).Error; err != nil {
		t.Fatalf("创建带安全库存物料失败: %v", err)
	}
	return mat
}

// inbound 用仓库服务向夹具仓/库位入库。
func inbound(t *testing.T, svc *warehouseService.WarehouseService, materialCode string, fx testsupport.Fixture, qty, unitCost string) {
	t.Helper()
	if _, err := svc.Inbound(testsupport.Ctx(), warehouseDto.InboundScanReq{
		MaterialCode:  materialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           testsupport.D(qty),
		UnitCost:      testsupport.D(unitCost),
	}); err != nil {
		t.Fatalf("入库失败(%s qty=%s): %v", materialCode, qty, err)
	}
}
