// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	stderrors "errors"

	"github.com/shopspring/decimal"

	apperrors "erp-system/internal/pkg/errors"
	productionDto "erp-system/internal/production/dto"
	"erp-system/internal/testsupport"
	warehouseDto "erp-system/internal/warehouse/dto"
)

func asAppErr(err error, target **apperrors.AppError) bool {
	return stderrors.As(err, target)
}

func inboundFor(fx testsupport.Fixture, qty, unitCost decimal.Decimal) warehouseDto.InboundScanReq {
	return warehouseDto.InboundScanReq{
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           qty,
		UnitCost:      unitCost,
	}
}

func issueReq(workOrderNo string, fx testsupport.Fixture, qty decimal.Decimal) productionDto.MaterialIssueScanReq {
	return productionDto.MaterialIssueScanReq{
		WorkOrderNo:   workOrderNo,
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           qty,
	}
}

// receiptReq 完工入库请求：成本由后端从工单成本池结转（P1.5 S1），不再传 unit_cost。
func receiptReq(workOrderNo string, fx testsupport.Fixture, qty decimal.Decimal) productionDto.ProductionReceiptReq {
	return productionDto.ProductionReceiptReq{
		WorkOrderNo:   workOrderNo,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           qty,
	}
}
