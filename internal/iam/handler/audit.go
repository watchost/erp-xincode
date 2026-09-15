// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"github.com/gin-gonic/gin"

	"erp-system/internal/iam/dto"
	"erp-system/internal/pkg/response"
)

// ListAuditLogs 审计日志分页查询（iam:audit:view）。
func (h *IAMHandler) ListAuditLogs(c *gin.Context) {
	var req dto.AuditLogListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	list, total, err := h.authService.ListAuditLogs(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.PageSuccess(c, list, total, req.Page, req.PageSize)
}
