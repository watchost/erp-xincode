// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"github.com/gin-gonic/gin"

	"erp-system/internal/iam/dto"
	"erp-system/internal/pkg/response"
)

func (h *IAMHandler) ListRoles(c *gin.Context) {
	list, err := h.authService.ListRoles(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, list)
}

func (h *IAMHandler) GetRoleDetail(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	detail, err := h.authService.GetRoleDetail(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, detail)
}

func (h *IAMHandler) CreateRole(c *gin.Context) {
	var req dto.CreateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.authService.CreateRole(c.Request.Context(), req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, nil)
}

func (h *IAMHandler) UpdateRole(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	var req dto.UpdateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.authService.UpdateRole(c.Request.Context(), id, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *IAMHandler) DeleteRole(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	if err := h.authService.DeleteRole(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// AssignPermissions 全量替换角色权限。
func (h *IAMHandler) AssignPermissions(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	var req dto.AssignPermsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.authService.AssignPermissions(c.Request.Context(), id, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// AssignRoleDepts 配置 data_scope=5 自定义部门集合。
func (h *IAMHandler) AssignRoleDepts(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	var req dto.AssignRoleDeptsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.authService.AssignRoleDepts(c.Request.Context(), id, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// PermissionTree 返回全量权限树，供角色授权页勾选。
func (h *IAMHandler) PermissionTree(c *gin.Context) {
	tree, err := h.authService.PermissionTree(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, tree)
}
