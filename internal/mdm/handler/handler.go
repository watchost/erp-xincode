// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"net/http"
	"strconv"

	"erp-system/internal/mdm/dto"
	"erp-system/internal/mdm/model"
	"erp-system/internal/mdm/service"
	"erp-system/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type MDMHandler struct {
	mdmService *service.MDMService
}

func NewMDMHandler(mdmService *service.MDMService) *MDMHandler {
	return &MDMHandler{mdmService: mdmService}
}

func (h *MDMHandler) ListMaterials(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	req := dto.MaterialListReq{
		MaterialCode: c.Query("material_code"),
		Name:         c.Query("name"),
		Page:         page,
		PageSize:     pageSize,
	}

	list, total, err := h.mdmService.ListMaterials(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

func (h *MDMHandler) GetMaterial(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	m, err := h.mdmService.GetMaterial(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(m))
}

func (h *MDMHandler) CreateMaterial(c *gin.Context) {
	var m model.MdmMaterial
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	err := h.mdmService.CreateMaterial(c.Request.Context(), &m)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(nil))
}

// UpdateMaterial 绑定指针字段 DTO（P1.6 H6），ID 取自路径参数；
// 未传字段（尤其 conversion_rate）由 service 保留原值，不再写 NULL。
func (h *MDMHandler) UpdateMaterial(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req dto.MaterialUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	err := h.mdmService.UpdateMaterial(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *MDMHandler) DeleteMaterial(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.mdmService.DeleteMaterial(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *MDMHandler) ListSuppliers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	req := dto.SupplierListReq{
		SupplierCode: c.Query("supplier_code"),
		Name:         c.Query("name"),
		Page:         page,
		PageSize:     pageSize,
	}

	list, total, err := h.mdmService.ListSuppliers(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

func (h *MDMHandler) GetSupplier(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	s, err := h.mdmService.GetSupplier(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(s))
}

func (h *MDMHandler) CreateSupplier(c *gin.Context) {
	var s model.MdmSupplier
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	err := h.mdmService.CreateSupplier(c.Request.Context(), &s)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *MDMHandler) UpdateSupplier(c *gin.Context) {
	var s model.MdmSupplier
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	err := h.mdmService.UpdateSupplier(c.Request.Context(), &s)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *MDMHandler) DeleteSupplier(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.mdmService.DeleteSupplier(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *MDMHandler) CreateWarehouse(c *gin.Context) {
	var w model.MdmWarehouse
	if err := c.ShouldBindJSON(&w); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	if err := h.mdmService.CreateWarehouse(c.Request.Context(), &w); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *MDMHandler) ListWarehouses(c *gin.Context) {
	list, err := h.mdmService.ListWarehouses(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(list))
}

func (h *MDMHandler) GetWarehouse(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	w, err := h.mdmService.GetWarehouse(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(w))
}

func (h *MDMHandler) ListLocations(c *gin.Context) {
	warehouseID, _ := strconv.ParseInt(c.Query("warehouse_id"), 10, 64)
	list, err := h.mdmService.ListLocations(c.Request.Context(), warehouseID)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(list))
}

func (h *MDMHandler) UpdateWarehouse(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var w model.MdmWarehouse
	if err := c.ShouldBindJSON(&w); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	w.ID = id
	if err := h.mdmService.UpdateWarehouse(c.Request.Context(), &w); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *MDMHandler) DeleteWarehouse(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.mdmService.DeleteWarehouse(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *MDMHandler) GetLocation(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	l, err := h.mdmService.GetLocation(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(l))
}

func (h *MDMHandler) CreateLocation(c *gin.Context) {
	var l model.MdmLocation
	if err := c.ShouldBindJSON(&l); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	if err := h.mdmService.CreateLocation(c.Request.Context(), &l); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *MDMHandler) UpdateLocation(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var l model.MdmLocation
	if err := c.ShouldBindJSON(&l); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	l.ID = id
	if err := h.mdmService.UpdateLocation(c.Request.Context(), &l); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *MDMHandler) DeleteLocation(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.mdmService.DeleteLocation(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}
