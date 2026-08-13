// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"github.com/gin-gonic/gin"

	"erp-system/internal/pkg/middleware"
	"erp-system/internal/pkg/response"
	"erp-system/internal/warehouse/dto"
	"erp-system/internal/warehouse/service"
)

type WarehouseHandler struct {
	warehouseService *service.WarehouseService
}

func NewWarehouseHandler(warehouseService *service.WarehouseService) *WarehouseHandler {
	return &WarehouseHandler{warehouseService: warehouseService}
}

func (h *WarehouseHandler) InboundScan(c *gin.Context) {
	var req dto.InboundScanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	req.IdempotencyKey = idempotencyKey(c)

	res, err := h.warehouseService.Inbound(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, res)
}

func (h *WarehouseHandler) OutboundScan(c *gin.Context) {
	var req dto.OutboundScanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	req.IdempotencyKey = idempotencyKey(c)

	res, err := h.warehouseService.Outbound(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, res)
}

func (h *WarehouseHandler) ListInventory(c *gin.Context) {
	var req dto.InventoryQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	list, total, err := h.warehouseService.ListInventory(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.PageSuccess(c, list, total, req.Page, req.PageSize)
}

func (h *WarehouseHandler) GetStockAlerts(c *gin.Context) {
	list, err := h.warehouseService.GetStockAlerts(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, list)
}

// 写入类接口应带 Idempotency-Key；未带时服务端不强制（可由路由/网关层要求）。
func idempotencyKey(c *gin.Context) string {
	if v := c.GetHeader("Idempotency-Key"); v != "" {
		return v
	}
	// 兼容旧客户端：X-Request-Id / X-Trace-ID 兜底
	if v := c.GetHeader("X-Request-Id"); v != "" {
		return v
	}
	if v, ok := c.Get(middleware.CtxJTI); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

