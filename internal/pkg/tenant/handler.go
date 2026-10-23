// Copyright 2026 zhouhouping. All Rights Reserved.

package tenant

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"erp-system/internal/pkg/auth"
	"erp-system/internal/pkg/response"
)

// Guard 挂在 JWT 之后：租户停用 → 全部拒绝；租户过期 → 拒绝写操作（GET 仍可读，便于导出数据）。
func Guard(s *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tid, ok := auth.TenantFromContext(c.Request.Context())
		if !ok {
			c.Next()
			return
		}
		write := c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && c.Request.Method != http.MethodOptions
		if err := s.Check(c.Request.Context(), tid, write); err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, response.FromError(err))
			return
		}
		c.Next()
	}
}

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func idParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 0 {
		c.JSON(http.StatusBadRequest, response.Err(10005, "租户 ID 不合法"))
		return 0, false
	}
	return id, true
}

func reply(c *gin.Context, data any, err error) {
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(data))
}

func (h *Handler) List(c *gin.Context) {
	list, err := h.svc.List(c.Request.Context())
	reply(c, list, err)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	t, err := h.svc.Create(c.Request.Context(), req)
	reply(c, t, err)
}

func (h *Handler) Update(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req UpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	reply(c, nil, h.svc.Update(c.Request.Context(), id, req))
}

func (h *Handler) Disable(c *gin.Context) {
	if id, ok := idParam(c); ok {
		reply(c, nil, h.svc.SetStatus(c.Request.Context(), id, StatusDisabled))
	}
}

func (h *Handler) Enable(c *gin.Context) {
	if id, ok := idParam(c); ok {
		reply(c, nil, h.svc.SetStatus(c.Request.Context(), id, StatusEnabled))
	}
}

func (h *Handler) Stats(c *gin.Context) {
	if id, ok := idParam(c); ok {
		vo, err := h.svc.Stats(c.Request.Context(), id)
		reply(c, vo, err)
	}
}

func (h *Handler) Current(c *gin.Context) {
	t, err := h.svc.Current(c.Request.Context())
	reply(c, t, err)
}

func (h *Handler) UpdateCurrent(c *gin.Context) {
	var req UpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	reply(c, nil, h.svc.UpdateCurrentConfig(c.Request.Context(), req))
}
