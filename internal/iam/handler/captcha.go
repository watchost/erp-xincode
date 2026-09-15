// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"github.com/gin-gonic/gin"

	"erp-system/internal/pkg/response"
)

// GetCaptcha 返回图形验证码（data URI）；连续登录失败 3 次后必传。
func (h *IAMHandler) GetCaptcha(c *gin.Context) {
	res, err := h.authService.GetCaptcha(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, res)
}
