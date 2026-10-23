// Copyright 2026 zhouhouping. All Rights Reserved.

package message

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"erp-system/internal/pkg/response"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

// List GET /messages?category=&unread=1&page=&page_size=
func (h *Handler) List(c *gin.Context) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	sz, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	unread := c.Query("unread") == "1" || c.Query("unread") == "true"
	list, total, err := h.svc.List(c.Request.Context(), c.Query("category"), unread, p, sz)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.Page(list, total, p, sz))
}

func (h *Handler) UnreadCount(c *gin.Context) {
	total, by, err := h.svc.UnreadCount(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"total": total, "by_category": by}))
}

func (h *Handler) MarkRead(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, response.Err(10005, "消息 ID 不合法"))
		return
	}
	if err := h.svc.MarkRead(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *Handler) MarkAllRead(c *gin.Context) {
	n, err := h.svc.MarkAllRead(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"updated": n}))
}

func (h *Handler) ListSubscriptions(c *gin.Context) {
	list, err := h.svc.ListSubscriptions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(list))
}

func (h *Handler) UpsertSubscription(c *gin.Context) {
	var req SubscriptionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	if err := h.svc.UpsertSubscription(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}
