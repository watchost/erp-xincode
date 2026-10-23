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

// ----------------- P2 §3.2 库存盘点 Handler -----------------

func (h *WarehouseHandler) CreateStockCheck(c *gin.Context) {
	var req dto.CreateStockCheckReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	userID := c.GetInt64(middleware.CtxUserID)
	res, err := h.warehouseService.CreateStockCheck(c.Request.Context(), req, userID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, res)
}

func (h *WarehouseHandler) FreezeStockCheck(c *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.warehouseService.FreezeStockCheck(c.Request.Context(), uri.ID); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, gin.H{"message": "仓库已冻结并进入盘点"})
}

func (h *WarehouseHandler) EntryStockCheck(c *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	var req dto.EntryStockCheckReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.warehouseService.EntryStockCheck(c.Request.Context(), uri.ID, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, gin.H{"message": "实盘录入成功"})
}

func (h *WarehouseHandler) ApproveStockCheck(c *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	userID := c.GetInt64(middleware.CtxUserID)
	if err := h.warehouseService.ApproveStockCheck(c.Request.Context(), uri.ID, userID); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, gin.H{"message": "盘点审批成功，库存已自动调账"})
}

func (h *WarehouseHandler) CancelStockCheck(c *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.warehouseService.CancelStockCheck(c.Request.Context(), uri.ID); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, gin.H{"message": "盘点单已取消，仓库已解冻"})
}

func (h *WarehouseHandler) GetStockCheck(c *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	res, err := h.warehouseService.GetStockCheck(c.Request.Context(), uri.ID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, res)
}

func (h *WarehouseHandler) ListStockChecks(c *gin.Context) {
	var req dto.StockCheckQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	list, total, err := h.warehouseService.ListStockChecks(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.PageSuccess(c, list, total, req.Page, req.PageSize)
}

// ----------------- P2 §3.3 库存调拨 Handler -----------------

func (h *WarehouseHandler) CreateTransfer(c *gin.Context) {
	var req dto.CreateTransferReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	userID := c.GetInt64(middleware.CtxUserID)
	res, err := h.warehouseService.CreateTransfer(c.Request.Context(), req, userID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, res)
}

func (h *WarehouseHandler) ApproveTransfer(c *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	userID := c.GetInt64(middleware.CtxUserID)
	if err := h.warehouseService.ApproveTransfer(c.Request.Context(), uri.ID, userID); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, gin.H{"message": "调拨单审批成功"})
}

func (h *WarehouseHandler) ShipTransfer(c *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.warehouseService.ShipTransfer(c.Request.Context(), uri.ID); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, gin.H{"message": "调拨单调出发货成功，已扣减调出仓库存并计入在途"})
}

func (h *WarehouseHandler) ReceiveTransfer(c *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.warehouseService.ReceiveTransfer(c.Request.Context(), uri.ID); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, gin.H{"message": "调拨单调入收货成功，已入调入仓库存并冲在途"})
}

func (h *WarehouseHandler) CancelTransfer(c *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.warehouseService.CancelTransfer(c.Request.Context(), uri.ID); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, gin.H{"message": "调拨单已取消"})
}

func (h *WarehouseHandler) GetTransfer(c *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	res, err := h.warehouseService.GetTransfer(c.Request.Context(), uri.ID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, res)
}

func (h *WarehouseHandler) ListTransfers(c *gin.Context) {
	var req dto.TransferQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	list, total, err := h.warehouseService.ListTransfers(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.PageSuccess(c, list, total, req.Page, req.PageSize)
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
