// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"erp-system/internal/pkg/response"
	"erp-system/internal/production/dto"
	"erp-system/internal/production/service"
)

type ProductionHandler struct {
	productionService *service.ProductionService
}

func NewProductionHandler(productionService *service.ProductionService) *ProductionHandler {
	return &ProductionHandler{productionService: productionService}
}

func (h *ProductionHandler) CreateWorkOrder(c *gin.Context) {
	var req dto.CreateWorkOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	vo, err := h.productionService.CreateWorkOrder(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, vo)
}

func (h *ProductionHandler) ReleaseWorkOrder(c *gin.Context) {
	workOrderNo := c.Param("work_order_no")
	if workOrderNo == "" {
		response.BadRequest(c, "工单号不能为空")
		return
	}
	if err := h.productionService.ReleaseWorkOrder(c.Request.Context(), workOrderNo); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *ProductionHandler) CloseWorkOrder(c *gin.Context) {
	workOrderNo := c.Param("work_order_no")
	if workOrderNo == "" {
		response.BadRequest(c, "工单号不能为空")
		return
	}
	if err := h.productionService.CloseWorkOrder(c.Request.Context(), workOrderNo); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *ProductionHandler) ListWorkOrders(c *gin.Context) {
	var req dto.WorkOrderListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	list, total, err := h.productionService.ListWorkOrders(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.PageSuccess(c, list, total, req.Page, req.PageSize)
}

func (h *ProductionHandler) MaterialIssueScan(c *gin.Context) {
	var req dto.MaterialIssueScanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	req.IdempotencyKey = c.GetHeader("Idempotency-Key")
	if req.IdempotencyKey == "" {
		response.BadRequest(c, "缺少 Idempotency-Key 请求头")
		return
	}

	res, err := h.productionService.MaterialIssueScan(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, res)
}

func (h *ProductionHandler) ProductionReceipt(c *gin.Context) {
	var req dto.ProductionReceiptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	req.IdempotencyKey = c.GetHeader("Idempotency-Key")
	if req.IdempotencyKey == "" {
		response.BadRequest(c, "缺少 Idempotency-Key 请求头")
		return
	}

	res, err := h.productionService.ProductionReceipt(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, res)
}

func (h *ProductionHandler) CreateBom(c *gin.Context) {
	var req dto.CreateBomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	vo, err := h.productionService.CreateBom(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, vo)
}

// ----------------- P2 §3.6 BOM 成本卷积与标准成本 Handler -----------------

func (h *ProductionHandler) ExplodeBom(c *gin.Context) {
	var uri struct {
		ProductID int64 `uri:"product_id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	tree, err := h.productionService.ExplodeBom(c.Request.Context(), uri.ProductID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, tree)
}

func (h *ProductionHandler) CalculateBomCostRollup(c *gin.Context) {
	var uri struct {
		ProductID int64 `uri:"product_id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	var req struct {
		LaborCost    decimal.Decimal `json:"labor_cost"`
		OverheadCost decimal.Decimal `json:"overhead_cost"`
	}
	_ = c.ShouldBindJSON(&req)

	res, err := h.productionService.CalculateBomCostRollup(c.Request.Context(), uri.ProductID, req.LaborCost, req.OverheadCost)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, res)
}

func (h *ProductionHandler) GetCostVariance(c *gin.Context) {
	var req dto.CostVarianceReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	res, err := h.productionService.GetCostVariance(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, res)
}

