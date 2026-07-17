// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"erp-system/internal/warehouse/dto"
	"erp-system/internal/warehouse/service"
	"erp-system/internal/pkg/response"
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
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	res, err := h.warehouseService.Inbound(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(res))
}

func (h *WarehouseHandler) OutboundScan(c *gin.Context) {
	var req dto.OutboundScanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	res, err := h.warehouseService.Outbound(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(res))
}

func (h *WarehouseHandler) ListInventory(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	warehouseID, _ := strconv.ParseInt(c.Query("warehouse_id"), 10, 64)

	req := dto.InventoryQuery{
		WarehouseID:   warehouseID,
		MaterialCode:  c.Query("material_code"),
		MaterialName:  c.Query("material_name"),
		Page:         page,
		PageSize:     pageSize,
	}

	list, total, err := h.warehouseService.ListInventory(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

func (h *WarehouseHandler) GetStockAlerts(c *gin.Context) {
	list, err := h.warehouseService.GetStockAlerts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(list))
}
