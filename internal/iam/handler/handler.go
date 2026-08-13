// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"erp-system/internal/iam/dto"
	"erp-system/internal/iam/service"
	"erp-system/internal/pkg/middleware"
	"erp-system/internal/pkg/response"
)

type IAMHandler struct {
	authService  *service.IAMService
	accessExpire time.Duration
}

func NewIAMHandler(authService *service.IAMService, accessExpire time.Duration) *IAMHandler {
	return &IAMHandler{authService: authService, accessExpire: accessExpire}
}

func (h *IAMHandler) Login(c *gin.Context) {
	var req dto.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	res, err := h.authService.Login(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, res)
}

func (h *IAMHandler) RefreshToken(c *gin.Context) {
	var req dto.RefreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	res, err := h.authService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, res)
}

// Logout 将当前 access token 的 jti 拉黑；前端应同时丢弃 refresh token。
func (h *IAMHandler) Logout(c *gin.Context) {
	jti, _ := c.Get(middleware.CtxJTI)
	jtiStr, _ := jti.(string)
	if err := h.authService.Logout(c.Request.Context(), jtiStr, h.accessExpire); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *IAMHandler) ChangePassword(c *gin.Context) {
	var req dto.ChangePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	userID := c.GetInt64(middleware.CtxUserID)
	if err := h.authService.ChangePassword(c.Request.Context(), userID, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *IAMHandler) GetUserInfo(c *gin.Context) {
	userID := c.GetInt64(middleware.CtxUserID)
	user, err := h.authService.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, dto.UserVO{
		ID:        user.ID,
		Username:  user.Username,
		RealName:  user.RealName,
		Phone:     user.Phone,
		Status:    user.Status,
		TenantID:  user.TenantID,
		CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

func (h *IAMHandler) GetPermissions(c *gin.Context) {
	userID := c.GetInt64(middleware.CtxUserID)
	perms, err := h.authService.GetUserPermissions(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if perms == nil {
		perms = []string{}
	}
	response.Success(c, perms)
}

func (h *IAMHandler) ListUsers(c *gin.Context) {
	var req dto.UserListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	list, total, err := h.authService.ListUsers(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.PageSuccess(c, list, total, req.Page, req.PageSize)
}

func (h *IAMHandler) GetUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "无效的用户ID")
		return
	}
	user, err := h.authService.GetUser(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, user)
}

func (h *IAMHandler) CreateUser(c *gin.Context) {
	var req dto.CreateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.authService.CreateUser(c.Request.Context(), req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, nil)
}

func (h *IAMHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "无效的用户ID")
		return
	}
	var req dto.UpdateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.authService.UpdateUser(c.Request.Context(), id, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}
