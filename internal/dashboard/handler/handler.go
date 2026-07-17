// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"erp-system/internal/dashboard/service"
	"erp-system/internal/pkg/response"
)

type DashboardHandler struct {
	dashboardService *service.DashboardService
}

func NewDashboardHandler(dashboardService *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{dashboardService: dashboardService}
}

func (h *DashboardHandler) GetOverview(c *gin.Context) {
	data, err := h.dashboardService.GetOverview(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(data))
}

func (h *DashboardHandler) GetStockAlerts(c *gin.Context) {
	data, err := h.dashboardService.GetStockAlerts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(data))
}

func (h *DashboardHandler) GetRecentOrders(c *gin.Context) {
	data, err := h.dashboardService.GetRecentOrders(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(data))
}

func (h *DashboardHandler) GetLLMAnalysis(c *gin.Context) {
	var req struct {
		Question string `json:"question" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误"))
		return
	}

	data, err := h.dashboardService.GetLLMAnalysis(c.Request.Context(), req.Question)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(data))
}
