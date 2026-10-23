// Copyright 2026 zhouhouping. All Rights Reserved.

package message

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ---- 事件适配：审批待办 ----

// ApprovalTodoTx 实现 workflow.TodoNotifier：新建待办（含转办、加签）时给审批人发站内信。
func (s *Service) ApprovalTodoTx(ctx context.Context, tx *gorm.DB, tenantID int64, assignees []int64,
	bizType, bizNo, nodeName, instanceNo string) error {
	return s.NotifyTx(ctx, tx, Event{
		TenantID:  tenantID,
		Category:  CategoryApprovalTodo,
		Receivers: assignees,
		Title:     fmt.Sprintf("待审批：%s %s", bizTypeName(bizType), bizNo),
		Content:   fmt.Sprintf("节点：%s", nodeName),
		BizType:   bizType,
		BizNo:     bizNo,
		Link:      "/workflow/instances/" + instanceNo,
	})
}

func bizTypeName(t string) string {
	switch t {
	case "purchase_order":
		return "采购订单"
	case "sales_order":
		return "销售订单"
	}
	return t
}

// ---- 事件适配：库存低于安全库存 ----

// StockLowTx 实现 warehouse.StockAlerter：出库后物料跨仓总库存从「≥ 安全库存」跌破到「< 安全库存」时，
// 给本租户 warehouse/purchase 角色用户发站内信（都没有时发给 admin 角色）。
// 只在跌破的那一次提醒，已经低于安全库存后的后续出库不重复提醒。
// 安全库存取 mdm_material.min_stock，为空表示不预警。
func (s *Service) StockLowTx(ctx context.Context, tx *gorm.DB, materialID int64, outQty decimal.Decimal) error {
	var mat struct {
		TenantID     int64
		MaterialCode string
		Name         string
		MinStock     *decimal.Decimal
	}
	if err := tx.Table("mdm_material").Select("tenant_id, material_code, name, min_stock").
		Where("id = ?", materialID).Scan(&mat).Error; err != nil {
		return err
	}
	if mat.MinStock == nil || !mat.MinStock.IsPositive() {
		return nil
	}
	var after decimal.Decimal
	if err := tx.Table("inv_inventory").Select("COALESCE(SUM(qty), 0)").
		Where("material_id = ?", materialID).Scan(&after).Error; err != nil {
		return err
	}
	before := after.Add(outQty)
	if !(before.GreaterThanOrEqual(*mat.MinStock) && after.LessThan(*mat.MinStock)) {
		return nil
	}
	receivers, err := s.usersByRoles(tx, mat.TenantID, "warehouse", "purchase")
	if err != nil {
		return err
	}
	if len(receivers) == 0 {
		if receivers, err = s.usersByRoles(tx, mat.TenantID, "admin"); err != nil {
			return err
		}
	}
	return s.NotifyTx(ctx, tx, Event{
		TenantID:  mat.TenantID,
		Category:  CategoryStockLow,
		Receivers: receivers,
		Title:     fmt.Sprintf("库存预警：%s %s 低于安全库存", mat.MaterialCode, mat.Name),
		Content:   fmt.Sprintf("当前库存 %s，安全库存 %s", after.String(), mat.MinStock.String()),
		BizType:   "material",
		BizNo:     mat.MaterialCode,
		Link:      fmt.Sprintf("/warehouse/inventory?material_id=%d", materialID),
	})
}

// usersByRoles 本租户内拥有指定角色、启用且未删除的用户。sys_user_role 没有 tenant_id，必须经 sys_user 限定租户。
func (s *Service) usersByRoles(tx *gorm.DB, tenantID int64, roles ...string) ([]int64, error) {
	var ids []int64
	err := tx.Table("sys_user u").Select("DISTINCT u.id").
		Joins("JOIN sys_user_role ur ON ur.user_id = u.id").
		Joins("JOIN sys_role r ON r.id = ur.role_id").
		Where("u.tenant_id = ? AND u.status = 1 AND u.deleted_at IS NULL AND r.code IN ?", tenantID, roles).
		Order("u.id").Scan(&ids).Error
	return ids, err
}
