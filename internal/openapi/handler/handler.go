// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"erp-system/internal/openapi/dto"
	"erp-system/internal/openapi/service"
	"erp-system/internal/pkg/response"
)

type OpenAPIHandler struct {
	openAPIService *service.OpenAPIService
}

func NewOpenAPIHandler(openAPIService *service.OpenAPIService) *OpenAPIHandler {
	return &OpenAPIHandler{openAPIService: openAPIService}
}

func (h *OpenAPIHandler) GetToken(c *gin.Context) {
	var req dto.TokenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	res, err := h.openAPIService.GetToken(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(res))
}

func (h *OpenAPIHandler) RefreshToken(c *gin.Context) {
	refreshToken := c.PostForm("refresh_token")
	if refreshToken == "" {
		c.JSON(http.StatusBadRequest, response.Err(10005, "缺少refresh_token"))
		return
	}

	res, err := h.openAPIService.RefreshToken(c.Request.Context(), refreshToken)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(res))
}

func (h *OpenAPIHandler) CreateWebhook(c *gin.Context) {
	var req dto.WebhookReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	res, err := h.openAPIService.CreateWebhook(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(res))
}

func (h *OpenAPIHandler) Sync(c *gin.Context) {
	var req dto.SyncReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	res, err := h.openAPIService.Sync(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(res))
}

func (h *OpenAPIHandler) ListClients(c *gin.Context) {
	_ = c.DefaultQuery("page", "1")
	_ = c.DefaultQuery("page_size", "20")

	c.JSON(http.StatusOK, response.OK([]interface{}{}))
}
