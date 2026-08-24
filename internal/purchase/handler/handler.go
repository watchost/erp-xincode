// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"github.com/gin-gonic/gin"

	"erp-system/internal/pkg/response"
	"erp-system/internal/purchase/dto"
	"erp-system/internal/purchase/service"
)

type PurchaseHandler struct {
	purchaseService *service.PurchaseService
}

func NewPurchaseHandler(purchaseService *service.PurchaseService) *PurchaseHandler {
	return &PurchaseHandler{purchaseService: purchaseService}
}

func (h *PurchaseHandler) CreateOrder(c *gin.Context) {
	var req dto.CreatePurchaseOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	vo, err := h.purchaseService.CreateOrder(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, vo)
}

func (h *PurchaseHandler) ApproveOrder(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "订单号不能为空")
		return
	}
	if err := h.purchaseService.ApproveOrder(c.Request.Context(), orderNo); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *PurchaseHandler) ListOrders(c *gin.Context) {
	var req dto.PurchaseOrderListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	list, total, err := h.purchaseService.ListOrders(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.PageSuccess(c, list, total, req.Page, req.PageSize)
}

func (h *PurchaseHandler) InboundScan(c *gin.Context) {
	var req dto.PurchaseInboundScanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	req.IdempotencyKey = c.GetHeader("Idempotency-Key")
	if req.IdempotencyKey == "" {
		response.BadRequest(c, "缺少 Idempotency-Key 请求头")
		return
	}

	res, err := h.purchaseService.InboundScan(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, res)
}
