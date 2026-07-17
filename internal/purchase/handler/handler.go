// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"erp-system/internal/purchase/dto"
	"erp-system/internal/purchase/service"
	"erp-system/internal/pkg/response"
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
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	vo, err := h.purchaseService.CreateOrder(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(vo))
}

func (h *PurchaseHandler) ApproveOrder(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		c.JSON(http.StatusBadRequest, response.Err(10005, "订单号不能为空"))
		return
	}

	if err := h.purchaseService.ApproveOrder(c.Request.Context(), orderNo); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *PurchaseHandler) ListOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	supplierID, _ := strconv.ParseInt(c.Query("supplier_id"), 10, 64)
	status, _ := strconv.Atoi(c.Query("status"))

	req := dto.PurchaseOrderListReq{
		OrderNo:    c.Query("order_no"),
		SupplierID: supplierID,
		Status:     status,
		Page:       page,
		PageSize:   pageSize,
	}

	list, total, err := h.purchaseService.ListOrders(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

func (h *PurchaseHandler) InboundScan(c *gin.Context) {
	var req dto.PurchaseInboundScanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	res, err := h.purchaseService.InboundScan(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(res))
}
