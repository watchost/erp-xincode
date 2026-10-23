// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"io"
	"net/http"
	"strconv"

	"erp-system/internal/mdm/dto"
	"erp-system/internal/mdm/model"
	"erp-system/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *MDMHandler) GetCategoryTree(c *gin.Context) {
	tree, err := h.mdmService.ListCategoryTree(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(tree))
}

func (h *MDMHandler) CreateCategory(c *gin.Context) {
	var req dto.CategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	m := model.MdmMaterialCategory{Code: req.Code, Name: req.Name, ParentID: req.ParentID}
	if err := h.mdmService.CreateCategory(c.Request.Context(), &m); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"id": m.ID}))
}

func (h *MDMHandler) UpdateCategory(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req dto.CategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	m := model.MdmMaterialCategory{ID: id, Code: req.Code, Name: req.Name}
	if err := h.mdmService.UpdateCategory(c.Request.Context(), &m); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *MDMHandler) DeleteCategory(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.mdmService.DeleteCategory(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *MDMHandler) ListCustomers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	req := dto.CustomerListReq{
		CustomerCode: c.Query("customer_code"),
		Name:         c.Query("name"),
		Page:         page,
		PageSize:     pageSize,
	}

	list, total, err := h.mdmService.ListCustomers(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

func (h *MDMHandler) GetCustomer(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	cu, err := h.mdmService.GetCustomer(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(cu))
}

func (h *MDMHandler) CreateCustomer(c *gin.Context) {
	var m model.MdmCustomer
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	if err := h.mdmService.CreateCustomer(c.Request.Context(), &m); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"id": m.ID}))
}

// UpdateCustomer 绑定指针字段 DTO（P1.5 S5），区分「字段未传」与「显式传零值」，
// 未传字段由 service 保留原值。
func (h *MDMHandler) UpdateCustomer(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req dto.CustomerUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	if err := h.mdmService.UpdateCustomer(c.Request.Context(), id, req); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *MDMHandler) DeleteCustomer(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.mdmService.DeleteCustomer(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *MDMHandler) ExportCustomers(c *gin.Context) {
	req := dto.CustomerListReq{
		CustomerCode: c.Query("customer_code"),
		Name:         c.Query("name"),
	}
	data, err := h.mdmService.ExportCustomersCSV(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=customers.csv")
	c.Data(http.StatusOK, "text/csv; charset=utf-8", data)
}

func (h *MDMHandler) ImportCustomers(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "请上传 CSV 文件"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "读取文件失败"))
		return
	}
	count, err := h.mdmService.ImportCustomersCSV(c.Request.Context(), data)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"imported_count": count}))
}
