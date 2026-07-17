// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"erp-system/internal/production/dto"
	"erp-system/internal/production/service"
	"erp-system/internal/pkg/response"
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
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	vo, err := h.productionService.CreateWorkOrder(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(vo))
}

func (h *ProductionHandler) ReleaseWorkOrder(c *gin.Context) {
	workOrderNo := c.Param("work_order_no")
	if workOrderNo == "" {
		c.JSON(http.StatusBadRequest, response.Err(10005, "工单号不能为空"))
		return
	}

	if err := h.productionService.ReleaseWorkOrder(c.Request.Context(), workOrderNo); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *ProductionHandler) ListWorkOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	productID, _ := strconv.ParseInt(c.Query("product_id"), 10, 64)
	status, _ := strconv.Atoi(c.Query("status"))

	req := dto.WorkOrderListReq{
		WorkOrderNo: c.Query("work_order_no"),
		ProductID:   productID,
		Status:      status,
		Page:        page,
		PageSize:    pageSize,
	}

	list, total, err := h.productionService.ListWorkOrders(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

func (h *ProductionHandler) MaterialIssueScan(c *gin.Context) {
	var req dto.MaterialIssueScanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	res, err := h.productionService.MaterialIssueScan(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(res))
}

func (h *ProductionHandler) CreateBom(c *gin.Context) {
	var req dto.CreateBomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	vo, err := h.productionService.CreateBom(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(vo))
}
