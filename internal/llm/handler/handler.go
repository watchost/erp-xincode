// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"erp-system/internal/llm/dto"
	"erp-system/internal/llm/service"
	"erp-system/internal/pkg/middleware"
	"erp-system/internal/pkg/response"
)

type LLMHandler struct {
	llmService *service.LLMService
}

func NewLLMHandler(llmService *service.LLMService) *LLMHandler {
	return &LLMHandler{llmService: llmService}
}

func currentUserID(c *gin.Context) (int64, bool) {
	userID := c.GetInt64(middleware.CtxUserID)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, response.Err(10001, "未认证"))
		return 0, false
	}
	return userID, true
}

func (h *LLMHandler) Chat(c *gin.Context) {
	var req dto.ChatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	// P0：不能再 fallback 到 1，必须来自 JWT 中间件，防止越权。
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	res, err := h.llmService.Chat(c.Request.Context(), userID, req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, res)
}

// ChatStream SSE 流式对话：每个 data 帧是一段增量文本，最后一帧带 session_id。
func (h *LLMHandler) ChatStream(c *gin.Context) {
	var req dto.ChatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)

	flusher, canFlush := c.Writer.(http.Flusher)
	emit := func(chunk string) error {
		if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", strconv.Quote(chunk)); err != nil {
			return err
		}
		if canFlush {
			flusher.Flush()
		}
		return nil
	}

	sessionID, err := h.llmService.ChatStream(c.Request.Context(), userID, req, emit)
	if err != nil {
		fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", strconv.Quote(err.Error()))
		if canFlush {
			flusher.Flush()
		}
		return
	}
	fmt.Fprintf(c.Writer, "event: done\ndata: {\"session_id\":%d}\n\n", sessionID)
	if canFlush {
		flusher.Flush()
	}
}

func (h *LLMHandler) GetHistory(c *gin.Context) {
	sessionID, err := strconv.ParseInt(c.Param("session_id"), 10, 64)
	if err != nil || sessionID <= 0 {
		response.BadRequest(c, "无效的会话ID")
		return
	}
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	messages, err := h.llmService.GetSessionHistory(c.Request.Context(), userID, sessionID)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, messages)
}

func (h *LLMHandler) ListSessions(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	sessions, err := h.llmService.ListSessions(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, sessions)
}

// ---------- Prompt 模板管理 ----------

func (h *LLMHandler) CreatePromptTemplate(c *gin.Context) {
	var req dto.PromptTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	vo, err := h.llmService.CreatePromptTemplate(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, vo)
}

func (h *LLMHandler) UpdatePromptTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "无效的模板ID")
		return
	}
	var req dto.PromptTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	vo, err := h.llmService.UpdatePromptTemplate(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, vo)
}

func (h *LLMHandler) DeletePromptTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "无效的模板ID")
		return
	}
	if err := h.llmService.DeletePromptTemplate(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *LLMHandler) ListPromptTemplates(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := h.llmService.ListPromptTemplates(c.Request.Context(), c.Query("scene"), page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.PageSuccess(c, list, total, page, pageSize)
}
