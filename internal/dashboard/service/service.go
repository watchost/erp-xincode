// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"erp-system/internal/dashboard/dto"
	warehouseModel "erp-system/internal/warehouse/model"
	purchaseModel "erp-system/internal/purchase/model"
	productionModel "erp-system/internal/production/model"
	"erp-system/internal/pkg/db"
)

type DashboardService struct {
	txManager *db.TxManager
	db        *gorm.DB
}

func NewDashboardService(txManager *db.TxManager, db *gorm.DB) *DashboardService {
	return &DashboardService{txManager: txManager, db: db}
}

func (s *DashboardService) GetOverview(ctx context.Context) (*dto.OverviewVO, error) {
	var vo dto.OverviewVO

	today := time.Now().Format("2006-01-02")

	_ = s.db.Model(&warehouseModel.InvInventory{}).Select("COALESCE(SUM(qty), 0)").Scan(&vo.TotalInventory)
	_ = s.db.Model(&warehouseModel.InvInventory{}).Select("COALESCE(SUM(qty * avg_cost), 0)").Scan(&vo.InventoryValue)

	_ = s.db.Model(&warehouseModel.InvStockLedger{}).
		Where("biz_type = ? AND DATE(created_at) = ?", 1, today).
		Select("COALESCE(SUM(change_qty), 0)").Scan(&vo.TodayInboundQty)

	_ = s.db.Model(&warehouseModel.InvStockLedger{}).
		Where("biz_type = ? AND DATE(created_at) = ?", 2, today).
		Select("COALESCE(SUM(ABS(change_qty)), 0)").Scan(&vo.TodayOutboundQty)

	_ = s.db.Model(&purchaseModel.PurPurchaseOrder{}).
		Where("status = ?", 1).
		Count(&vo.PendingPurchaseOrders)

	_ = s.db.Model(&productionModel.ProWorkOrder{}).
		Where("status = ? OR status = ?", 2, 3).
		Count(&vo.InProgressWorkOrders)

	_ = s.db.Model(&warehouseModel.InvInventory{}).
		Where("available_qty < 10").
		Count(&vo.StockAlertCount)

	return &vo, nil
}

func (s *DashboardService) GetStockAlerts(ctx context.Context) ([]interface{}, error) {
	var alerts []interface{}
	return alerts, nil
}

func (s *DashboardService) GetRecentOrders(ctx context.Context) ([]interface{}, error) {
	var orders []interface{}
	return orders, nil
}

func (s *DashboardService) GetLLMAnalysis(ctx context.Context, question string) (*dto.LLMAnalysisVO, error) {
	answer := fmt.Sprintf("LLM分析服务暂未连接外部AI服务。您的问题是：%s", question)
	return &dto.LLMAnalysisVO{
		Question:    question,
		Answer:      answer,
		AnalysisType: "text",
	}, nil
}
