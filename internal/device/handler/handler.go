// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"erp-system/internal/device/dto"
	"erp-system/internal/device/service"
	"erp-system/internal/pkg/response"
)

type DeviceHandler struct {
	deviceService *service.DeviceService
}

func NewDeviceHandler(deviceService *service.DeviceService) *DeviceHandler {
	return &DeviceHandler{deviceService: deviceService}
}

func (h *DeviceHandler) Register(c *gin.Context) {
	var req dto.DeviceRegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	vo, err := h.deviceService.Register(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(vo))
}

func (h *DeviceHandler) Heartbeat(c *gin.Context) {
	var req dto.DeviceHeartbeatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	if err := h.deviceService.Heartbeat(c.Request.Context(), req.DeviceCode); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *DeviceHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	deviceType, _ := strconv.Atoi(c.Query("type"))
	status, _ := strconv.Atoi(c.Query("status"))

	req := dto.DeviceListReq{
		DeviceCode: c.Query("device_code"),
		Type:       deviceType,
		Status:     status,
		Page:       page,
		PageSize:   pageSize,
	}

	list, total, err := h.deviceService.List(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

func (h *DeviceHandler) GetByCode(c *gin.Context) {
	deviceCode := c.Param("device_code")
	vo, err := h.deviceService.GetByCode(c.Request.Context(), deviceCode)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(vo))
}
