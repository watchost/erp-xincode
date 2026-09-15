// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"github.com/gin-gonic/gin"

	"erp-system/internal/iam/dto"
	"erp-system/internal/pkg/response"
)

// DepartmentTree 部门树；登录即可访问（用户表单也要用）。
func (h *IAMHandler) DepartmentTree(c *gin.Context) {
	tree, err := h.authService.DepartmentTree(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, tree)
}

func (h *IAMHandler) CreateDepartment(c *gin.Context) {
	var req dto.DepartmentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.authService.CreateDepartment(c.Request.Context(), req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, nil)
}

func (h *IAMHandler) UpdateDepartment(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	var req dto.DepartmentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.authService.UpdateDepartment(c.Request.Context(), id, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *IAMHandler) DeleteDepartment(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	if err := h.authService.DeleteDepartment(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}
