// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"erp-system/internal/iam/dto"
	"erp-system/internal/iam/model"
	"erp-system/internal/iam/service"
	"erp-system/internal/pkg/response"
)

type IAMHandler struct {
	authService *service.IAMService
	jwtSecret   string
}

func NewIAMHandler(authService *service.IAMService, jwtSecret string) *IAMHandler {
	return &IAMHandler{authService: authService, jwtSecret: jwtSecret}
}

func (h *IAMHandler) Login(c *gin.Context) {
	var req dto.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	res, err := h.authService.Login(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(res))
}

func (h *IAMHandler) RefreshToken(c *gin.Context) {
	var req dto.RefreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	res, err := h.authService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(res))
}

func (h *IAMHandler) GetUserInfo(c *gin.Context) {
	userID := c.GetInt64("user_id")
	user, err := h.authService.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(dto.UserVO{
		ID:        user.ID,
		Username:  user.Username,
		RealName:  user.RealName,
		Phone:     user.Phone,
		Status:    user.Status,
		CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
	}))
}

func (h *IAMHandler) GetPermissions(c *gin.Context) {
	userID := c.GetInt64("user_id")
	perms, err := h.authService.GetUserPermissions(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(perms))
}

func (h *IAMHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	req := dto.UserListReq{
		Username: c.Query("username"),
		Page:     page,
		PageSize: pageSize,
	}

	list, total, err := h.authService.ListUsers(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

func (h *IAMHandler) GetUser(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	user, err := h.authService.GetUser(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(user))
}

func (h *IAMHandler) CreateUser(c *gin.Context) {
	var user model.SysUser
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	err := h.authService.CreateUser(c.Request.Context(), &user)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *IAMHandler) UpdateUser(c *gin.Context) {
	var user model.SysUser
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	err := h.authService.UpdateUser(c.Request.Context(), &user)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(nil))
}
