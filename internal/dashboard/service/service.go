// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"erp-system/internal/dashboard/dto"
	"erp-system/internal/llm/gateway"
	"erp-system/internal/pkg/db"
	productionModel "erp-system/internal/production/model"
	productionService "erp-system/internal/production/service"
	purchaseModel "erp-system/internal/purchase/model"
	purchaseService "erp-system/internal/purchase/service"
	warehouseModel "erp-system/internal/warehouse/model"
	"gorm.io/gorm"
)

// defaultAlertThreshold 物料未设置安全库存(min_stock)时的预警默认阈值。
// 库存预警计数与明细列表共用该阈值，保证 KPI 数字与列表条数一致。
const defaultAlertThreshold = 10

type DashboardService struct {
	txManager  *db.TxManager
	db         *gorm.DB
	llmGateway gateway.LLMGateway
	llmEnabled bool
}

func NewDashboardService(txManager *db.TxManager, db *gorm.DB, llmGateway gateway.LLMGateway, llmEnabled bool) *DashboardService {
	return &DashboardService{txManager: txManager, db: db, llmGateway: llmGateway, llmEnabled: llmEnabled}
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

	// P1.5 H7：迁移 0010 后状态码统一改版（旧 1/2/5/3/4 → 新 10/20/30/40/50），
	// 统计口径必须引用采购/生产模块导出的状态常量，禁止再硬编码旧码。
	_ = s.db.Model(&purchaseModel.PurPurchaseOrder{}).
		Where("status = ?", purchaseService.OrderStatusDraft).
		Count(&vo.PendingPurchaseOrders)

	_ = s.db.Model(&productionModel.ProWorkOrder{}).
		Where("status IN ?", []int{productionService.WorkOrderStatusReleased, productionService.WorkOrderStatusInProgress}).
		Count(&vo.InProgressWorkOrders)

	_ = s.alertBaseQuery().Count(&vo.StockAlertCount)

	return &vo, nil
}

// alertBaseQuery 库存预警明细的公共查询：库存连物料/仓库，
// 可用量低于「物料安全库存，未设置则默认阈值」。
func (s *DashboardService) alertBaseQuery() *gorm.DB {
	return s.db.Table("inv_inventory AS i").
		Joins("LEFT JOIN mdm_material AS m ON m.id = i.material_id AND m.deleted_at IS NULL").
		Joins("LEFT JOIN mdm_warehouse AS w ON w.id = i.warehouse_id AND w.deleted_at IS NULL").
		Where("i.available_qty < COALESCE(NULLIF(m.min_stock, 0), ?)", defaultAlertThreshold)
}

func (s *DashboardService) GetStockAlerts(ctx context.Context) ([]dto.StockAlertVO, error) {
	var rows []struct {
		MaterialCode  string
		MaterialName  string
		WarehouseName string
		Qty           decimal.Decimal
		AvailableQty  decimal.Decimal
		MinStock      *decimal.Decimal
	}
	err := s.alertBaseQuery().
		Select("m.material_code AS material_code, m.name AS material_name, w.name AS warehouse_name, " +
			"i.qty AS qty, i.available_qty AS available_qty, m.min_stock AS min_stock").
		Order("i.available_qty ASC").
		Limit(50).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	alerts := make([]dto.StockAlertVO, 0, len(rows))
	for _, r := range rows {
		threshold := decimal.NewFromInt(defaultAlertThreshold)
		if r.MinStock != nil && r.MinStock.IsPositive() {
			threshold = *r.MinStock
		}
		level := "warning"
		// 可用量不足阈值一半视为紧急
		if r.AvailableQty.Cmp(threshold.DivRound(decimal.NewFromInt(2), 3)) < 0 {
			level = "danger"
		}
		alerts = append(alerts, dto.StockAlertVO{
			MaterialCode:  r.MaterialCode,
			MaterialName:  r.MaterialName,
			WarehouseName: r.WarehouseName,
			CurrentQty:    r.Qty,
			AvailableQty:  r.AvailableQty,
			MinStock:      threshold,
			Level:         level,
		})
	}
	return alerts, nil
}

// purchaseStatusDesc 采购订单状态描述。P1.5 H7：迁移 0010 状态码改版后
// 同步新码（10/20/30/40/50），case 直接引用采购模块导出常量。
func purchaseStatusDesc(status int) string {
	switch status {
	case purchaseService.OrderStatusDraft:
		return "草稿"
	case purchaseService.OrderStatusApproved:
		return "已审批"
	case purchaseService.OrderStatusPartiallyReceived:
		return "部分收货"
	case purchaseService.OrderStatusCompleted:
		return "已完成"
	case purchaseService.OrderStatusCanceled:
		return "已取消"
	default:
		return "未知"
	}
}

func (s *DashboardService) GetRecentOrders(ctx context.Context) ([]dto.RecentOrderVO, error) {
	var rows []struct {
		OrderNo      string
		SupplierName string
		TotalAmount  decimal.Decimal
		Status       int
		CreatedAt    time.Time
	}
	err := s.db.Table("pur_purchase_order AS o").
		Joins("LEFT JOIN mdm_supplier AS sup ON sup.id = o.supplier_id AND sup.deleted_at IS NULL").
		Select("o.order_no AS order_no, COALESCE(sup.name, '') AS supplier_name, " +
			"o.total_amount AS total_amount, o.status AS status, o.created_at AS created_at").
		Order("o.created_at DESC").
		Limit(10).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	orders := make([]dto.RecentOrderVO, 0, len(rows))
	for _, r := range rows {
		orders = append(orders, dto.RecentOrderVO{
			OrderNo:      r.OrderNo,
			SupplierName: r.SupplierName,
			TotalAmount:  r.TotalAmount,
			StatusDesc:   purchaseStatusDesc(r.Status),
			CreatedAt:    r.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	return orders, nil
}

func (s *DashboardService) GetLLMAnalysis(ctx context.Context, question string) (*dto.LLMAnalysisVO, error) {
	// 以真实经营数据作为上下文，避免模型凭空回答
	overview, _ := s.GetOverview(ctx)
	alerts, _ := s.GetStockAlerts(ctx)
	orders, _ := s.GetRecentOrders(ctx)

	dataCtx := s.buildDataContext(overview, alerts, orders)
	answer := s.fallbackAnswer(question, overview, alerts)

	if s.llmEnabled && s.llmGateway != nil {
		messages := []gateway.Message{
			{Role: "system", Content: "你是 ERP 系统的智能分析助手，基于提供的真实经营数据给出简洁、专业的分析与建议，不要编造数据。"},
			{Role: "user", Content: dataCtx + "\n用户问题：" + question},
		}
		if ai, err := s.llmGateway.Chat(ctx, messages); err == nil && strings.TrimSpace(ai) != "" {
			answer = ai
		}
	}

	return &dto.LLMAnalysisVO{
		Question:     question,
		Answer:       answer,
		AnalysisType: "text",
	}, nil
}

// buildDataContext 把仪表盘真实数据整理成给模型的事实清单。
func (s *DashboardService) buildDataContext(ov *dto.OverviewVO, alerts []dto.StockAlertVO, orders []dto.RecentOrderVO) string {
	var b strings.Builder
	b.WriteString("【当前经营数据】\n")
	b.WriteString(fmt.Sprintf("- 库存总量: %s，库存总值: ¥%s\n", ov.TotalInventory.StringFixed(0), ov.InventoryValue.StringFixed(2)))
	b.WriteString(fmt.Sprintf("- 今日入库: %s，今日出库: %s\n", ov.TodayInboundQty.StringFixed(0), ov.TodayOutboundQty.StringFixed(0)))
	b.WriteString(fmt.Sprintf("- 待审批采购订单: %d，进行中工单: %d，库存预警: %d\n",
		ov.PendingPurchaseOrders, ov.InProgressWorkOrders, ov.StockAlertCount))

	if len(alerts) > 0 {
		b.WriteString("- 库存预警明细(物料/仓库/可用量): ")
		for i, a := range alerts {
			if i >= 5 {
				b.WriteString("等")
				break
			}
			if i > 0 {
				b.WriteString("；")
			}
			b.WriteString(fmt.Sprintf("%s(%s) 可用%s", a.MaterialName, a.WarehouseName, a.AvailableQty.StringFixed(0)))
		}
		b.WriteString("\n")
	}

	if len(orders) > 0 {
		b.WriteString("- 最近采购订单: ")
		for i, o := range orders {
			if i >= 5 {
				break
			}
			if i > 0 {
				b.WriteString("；")
			}
			b.WriteString(fmt.Sprintf("%s %s ¥%s(%s)", o.OrderNo, o.SupplierName, o.TotalAmount.StringFixed(2), o.StatusDesc))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// fallbackAnswer 未配置/调用 LLM 失败时，基于真实数据给出确定性摘要，不回显问题、不编造。
func (s *DashboardService) fallbackAnswer(question string, ov *dto.OverviewVO, alerts []dto.StockAlertVO) string {
	var b strings.Builder
	b.WriteString("（未连接外部 AI，以下为系统基于实时数据的摘要）\n")
	b.WriteString(fmt.Sprintf("当前库存总值约 ¥%s，库存总量 %s；今日入库 %s、出库 %s。",
		ov.InventoryValue.StringFixed(2), ov.TotalInventory.StringFixed(0), ov.TodayInboundQty.StringFixed(0), ov.TodayOutboundQty.StringFixed(0)))
	b.WriteString(fmt.Sprintf("待审批采购订单 %d 张，进行中工单 %d 张。", ov.PendingPurchaseOrders, ov.InProgressWorkOrders))

	if len(alerts) > 0 {
		b.WriteString(fmt.Sprintf("\n需优先关注 %d 项低库存：", len(alerts)))
		for i, a := range alerts {
			if i >= 3 {
				b.WriteString("等。")
				break
			}
			b.WriteString(fmt.Sprintf("\n· %s（%s）可用 %s，低于安全库存 %s；", a.MaterialName, a.WarehouseName, a.AvailableQty.StringFixed(0), a.MinStock.StringFixed(0)))
		}
		b.WriteString("\n建议尽快对上述物料下达采购或调整生产排程。")
	} else {
		b.WriteString("\n当前无库存预警，库存水位正常。")
	}
	return b.String()
}
