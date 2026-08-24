// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
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

func (h *LLMHandler) Chat(c *gin.Context) {
	var req dto.ChatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	// P0：不能再 fallback 到 1，必须来自 JWT 中间件，防止越权。
	userID := c.GetInt64(middleware.CtxUserID)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, response.Err(10001, "未认证"))
		return
	}

	res, err := h.llmService.Chat(c.Request.Context(), userID, req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, res)
}

func (h *LLMHandler) GetHistory(c *gin.Context) {
	sessionID, err := strconv.ParseInt(c.Param("session_id"), 10, 64)
	if err != nil || sessionID <= 0 {
		response.BadRequest(c, "无效的会话ID")
		return
	}

	messages, err := h.llmService.GetSessionHistory(c.Request.Context(), sessionID)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, messages)
}

func (h *LLMHandler) ListSessions(c *gin.Context) {
	userID := c.GetInt64(middleware.CtxUserID)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, response.Err(10001, "未认证"))
		return
	}

	sessions, err := h.llmService.ListSessions(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, sessions)
}
