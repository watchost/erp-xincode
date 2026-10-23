// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"github.com/gin-gonic/gin"

	"erp-system/internal/pkg/response"
	"erp-system/internal/sales/dto"
	"erp-system/internal/sales/service"
)

type SalesHandler struct {
	salesService *service.SalesService
}

func NewSalesHandler(salesService *service.SalesService) *SalesHandler {
	return &SalesHandler{salesService: salesService}
}

func (h *SalesHandler) CreateOrder(c *gin.Context) {
	var req dto.CreateSalesOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	vo, err := h.salesService.CreateOrder(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, vo)
}

func (h *SalesHandler) UpdateOrder(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "订单号不能为空")
		return
	}
	var req dto.UpdateSalesOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	vo, err := h.salesService.UpdateOrder(c.Request.Context(), orderNo, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, vo)
}

func (h *SalesHandler) GetOrder(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "订单号不能为空")
		return
	}
	vo, err := h.salesService.GetOrder(c.Request.Context(), orderNo)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, vo)
}

func (h *SalesHandler) ApproveOrder(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "订单号不能为空")
		return
	}
	if err := h.salesService.ApproveOrder(c.Request.Context(), orderNo); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *SalesHandler) CancelOrder(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "订单号不能为空")
		return
	}
	if err := h.salesService.CancelOrder(c.Request.Context(), orderNo); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *SalesHandler) ListOrders(c *gin.Context) {
	var req dto.SalesOrderListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	list, total, err := h.salesService.ListOrders(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.PageSuccess(c, list, total, req.Page, req.PageSize)
}

func (h *SalesHandler) OutboundScan(c *gin.Context) {
	var req dto.SalesOutboundScanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	req.IdempotencyKey = c.GetHeader("Idempotency-Key")
	if req.IdempotencyKey == "" {
		response.BadRequest(c, "缺少 Idempotency-Key 请求头")
		return
	}

	res, err := h.salesService.OutboundScan(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, res)
}

func (h *SalesHandler) ListOutbounds(c *gin.Context) {
	var req dto.SalesOutboundListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	list, total, err := h.salesService.ListOutbounds(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.PageSuccess(c, list, total, req.Page, req.PageSize)
}

func (h *SalesHandler) CreateReturn(c *gin.Context) {
	var req dto.CreateSalesReturnReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	vo, err := h.salesService.CreateReturn(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, vo)
}

// ReceiveReturnReq 退货入库的仓库/库位（退货实物回到哪个库位）。
type ReceiveReturnReq struct {
	WarehouseCode string `json:"warehouse_code" binding:"required"`
	LocationCode  string `json:"location_code" binding:"required"`
}

func (h *SalesHandler) ReceiveReturn(c *gin.Context) {
	returnNo := c.Param("return_no")
	if returnNo == "" {
		response.BadRequest(c, "退货单号不能为空")
		return
	}
	var req ReceiveReturnReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.salesService.ReceiveReturn(c.Request.Context(), returnNo, req.WarehouseCode, req.LocationCode); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *SalesHandler) GetReturn(c *gin.Context) {
	returnNo := c.Param("return_no")
	if returnNo == "" {
		response.BadRequest(c, "退货单号不能为空")
		return
	}
	vo, err := h.salesService.GetReturn(c.Request.Context(), returnNo)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, vo)
}
