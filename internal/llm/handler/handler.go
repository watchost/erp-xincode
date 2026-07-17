// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"erp-system/internal/llm/dto"
	"erp-system/internal/llm/service"
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

	userID := c.GetInt64("user_id")
	if userID == 0 {
		userID = 1
	}

	res, err := h.llmService.Chat(c.Request.Context(), userID, req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(res))
}

func (h *LLMHandler) GetHistory(c *gin.Context) {
	sessionID, _ := strconv.ParseInt(c.Param("session_id"), 10, 64)

	messages, err := h.llmService.GetSessionHistory(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(messages))
}

func (h *LLMHandler) ListSessions(c *gin.Context) {
	userID := c.GetInt64("user_id")
	if userID == 0 {
		userID = 1
	}

	sessions, err := h.llmService.ListSessions(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(sessions))
}
