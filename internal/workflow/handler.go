// Copyright 2026 zhouhouping. All Rights Reserved.

package workflow

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"erp-system/internal/pkg/response"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func bad(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
}

func taskID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, response.Err(10005, "任务 ID 不合法"))
		return 0, false
	}
	return id, true
}

func pageArgs(c *gin.Context) (int, int) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	s, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	return p, s
}

func (h *Handler) ListDefinitions(c *gin.Context) {
	list, err := h.svc.ListDefinitions(c.Request.Context(), c.Query("code"))
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(list))
}

func (h *Handler) CreateDefinition(c *gin.Context) {
	var req CreateDefinitionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	def, err := h.svc.CreateDefinition(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(def))
}

func (h *Handler) DeployDefinition(c *gin.Context) {
	id, ok := taskID(c)
	if !ok {
		return
	}
	if err := h.svc.DeployDefinition(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *Handler) Start(c *gin.Context) {
	var req StartReq
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	inst, err := h.svc.Start(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"instance_no": inst.InstanceNo, "status": inst.Status, "current_node": inst.CurrentNode}))
}

func (h *Handler) Todo(c *gin.Context) {
	p, s := pageArgs(c)
	list, total, err := h.svc.ListTodo(c.Request.Context(), p, s)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.Page(list, total, p, s))
}

func (h *Handler) Done(c *gin.Context) {
	p, s := pageArgs(c)
	list, total, err := h.svc.ListDone(c.Request.Context(), p, s)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.Page(list, total, p, s))
}

func (h *Handler) Approve(c *gin.Context) {
	id, ok := taskID(c)
	if !ok {
		return
	}
	var req ActionReq
	_ = c.ShouldBindJSON(&req)
	if err := h.svc.Approve(c.Request.Context(), id, req.Comment); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *Handler) Reject(c *gin.Context) {
	id, ok := taskID(c)
	if !ok {
		return
	}
	var req ActionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	if err := h.svc.Reject(c.Request.Context(), id, req.Comment); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *Handler) Transfer(c *gin.Context) {
	id, ok := taskID(c)
	if !ok {
		return
	}
	var req TargetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	if err := h.svc.Transfer(c.Request.Context(), id, req.TargetUserID, req.Comment); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *Handler) AddSign(c *gin.Context) {
	id, ok := taskID(c)
	if !ok {
		return
	}
	var req TargetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	if err := h.svc.AddSign(c.Request.Context(), id, req.TargetUserID, req.Comment); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *Handler) GetInstance(c *gin.Context) {
	vo, err := h.svc.GetInstance(c.Request.Context(), c.Param("no"))
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(vo))
}

func (h *Handler) Cancel(c *gin.Context) {
	var req ActionReq
	_ = c.ShouldBindJSON(&req)
	if err := h.svc.Cancel(c.Request.Context(), c.Param("no"), req.Comment); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

// ByBiz 按业务单据查询最近一次审批实例号：GET /workflow/instances/by-biz?biz_type=&biz_no=
func (h *Handler) ByBiz(c *gin.Context) {
	no, err := h.svc.LatestInstanceNo(c.Request.Context(), c.Query("biz_type"), c.Query("biz_no"))
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"instance_no": no}))
}
