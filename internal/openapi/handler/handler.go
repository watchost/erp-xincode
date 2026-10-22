// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"net/http"
	"strconv"

	"erp-system/internal/openapi/dto"
	openapiMiddleware "erp-system/internal/openapi/middleware"
	"erp-system/internal/openapi/service"
	"erp-system/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type OpenAPIHandler struct {
	openAPIService *service.OpenAPIService
	resourceSvc    *service.ResourceService
}

func NewOpenAPIHandler(openAPIService *service.OpenAPIService, resourceSvc *service.ResourceService) *OpenAPIHandler {
	return &OpenAPIHandler{openAPIService: openAPIService, resourceSvc: resourceSvc}
}

// clientID 从 BearerAuth 注入的上下文取 client_id（不接受 body 传入）。
func clientID(c *gin.Context) string { return c.GetString(openapiMiddleware.CtxClientID) }

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
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := c.ShouldBindJSON(&body); err == nil {
			refreshToken = body.RefreshToken
		}
	}
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

	res, err := h.openAPIService.CreateWebhook(c.Request.Context(), clientID(c), req)
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

	res, err := h.openAPIService.Sync(c.Request.Context(), clientID(c), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(res))
}

// ---------- 客户端管理（内部管理端） ----------

func (h *OpenAPIHandler) CreateClient(c *gin.Context) {
	var req dto.ClientCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	res, err := h.openAPIService.CreateClient(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

func (h *OpenAPIHandler) ListClients(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := h.openAPIService.ListClients(c.Request.Context(), page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

// ---------- 标准资源 API（/openapi/v1，Bearer + scope） ----------

func (h *OpenAPIHandler) ListMaterials(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := h.resourceSvc.ListMaterials(c.Request.Context(), c.Query("material_code"), page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

func (h *OpenAPIHandler) ListInventory(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := h.resourceSvc.ListInventory(c.Request.Context(), c.Query("material_code"), c.Query("warehouse_code"), page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

func (h *OpenAPIHandler) CreateOrder(c *gin.Context) {
	var req dto.OrderCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	vo, err := h.resourceSvc.CreateOrder(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(vo))
}

func (h *OpenAPIHandler) GetOrder(c *gin.Context) {
	vo, err := h.resourceSvc.GetOrder(c.Request.Context(), c.Param("order_no"))
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(vo))
}
