// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"erp-system/internal/iam/dto"
	"erp-system/internal/pkg/response"
)

func parsePathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "无效的ID")
		return 0, false
	}
	return id, true
}

// DeleteUser 软删除用户；删除后同名账号可重新创建。
func (h *IAMHandler) DeleteUser(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	if err := h.authService.DeleteUser(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

// ResetPassword 管理员重置密码；请求体为空时服务端生成 12 位随机密码并一次性返回。
func (h *IAMHandler) ResetPassword(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	var req dto.ResetPasswordReq
	// 允许空 body（生成随机密码）；仅有 body 且非法时才报错
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "参数错误: "+err.Error())
			return
		}
	}
	res, err := h.authService.ResetPassword(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, res)
}

// AssignUserRoles 全量替换用户角色。
func (h *IAMHandler) AssignUserRoles(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	var req dto.AssignUserRolesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.authService.AssignUserRoles(c.Request.Context(), id, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}
