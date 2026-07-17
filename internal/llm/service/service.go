// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"time"

	"erp-system/internal/llm/dto"
	"erp-system/internal/llm/gateway"
	"erp-system/internal/llm/model"
	"erp-system/internal/llm/repository"
)

type LLMService struct {
	sessionRepo  repository.SessionRepository
	messageRepo  repository.MessageRepository
	llmGateway   gateway.LLMGateway
}

func NewLLMService(
	sessionRepo repository.SessionRepository,
	messageRepo repository.MessageRepository,
	llmGateway gateway.LLMGateway,
) *LLMService {
	return &LLMService{
		sessionRepo: sessionRepo,
		messageRepo: messageRepo,
		llmGateway:  llmGateway,
	}
}

func (s *LLMService) Chat(ctx context.Context, userID int64, req dto.ChatReq) (*dto.ChatRes, error) {
	sessionID := req.SessionID
	if sessionID == 0 {
		session := &model.LlmSession{
			UserID:    userID,
			Title:     truncateTitle(req.Question, 50),
		}
		if err := s.sessionRepo.Create(session); err != nil {
			return nil, err
		}
		sessionID = session.ID
	}

	userMessage := &model.LlmMessage{
		SessionID: sessionID,
		Role:      model.RoleUser,
		Content:   req.Question,
	}
	if err := s.messageRepo.Create(userMessage); err != nil {
		return nil, err
	}

	messages, err := s.buildMessages(sessionID, req.Question)
	if err != nil {
		return nil, err
	}

	answer, err := s.llmGateway.Chat(ctx, messages)
	if err != nil {
		answer = "AI服务暂时不可用，请稍后重试"
	}

	assistantMessage := &model.LlmMessage{
		SessionID: sessionID,
		Role:      model.RoleAssistant,
		Content:   answer,
	}
	if err := s.messageRepo.Create(assistantMessage); err != nil {
		return nil, err
	}

	return &dto.ChatRes{
		SessionID: sessionID,
		Answer:    answer,
		Intent:    "analysis",
	}, nil
}

func (s *LLMService) buildMessages(sessionID int64, question string) ([]gateway.Message, error) {
	var messages []gateway.Message

	messages = append(messages, gateway.Message{
		Role:    "system",
		Content: "你是ERP系统的智能分析助手，帮助用户进行业务数据分析和决策支持。",
	})

	messages = append(messages, gateway.Message{
		Role:    "user",
		Content: question,
	})

	return messages, nil
}

func (s *LLMService) GetSessionHistory(ctx context.Context, sessionID int64) ([]dto.MessageVO, error) {
	messages, err := s.messageRepo.ListBySessionID(sessionID)
	if err != nil {
		return nil, err
	}

	var vos []dto.MessageVO
	for _, m := range messages {
		roleName := ""
		switch m.Role {
		case model.RoleUser:
			roleName = "用户"
		case model.RoleAssistant:
			roleName = "助手"
		case model.RoleSystem:
			roleName = "系统"
		}

		vos = append(vos, dto.MessageVO{
			ID:        m.ID,
			Role:      m.Role,
			RoleName:  roleName,
			Content:   m.Content,
			Intent:    m.Intent,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}

	return vos, nil
}

func (s *LLMService) ListSessions(ctx context.Context, userID int64) ([]dto.SessionVO, error) {
	sessions, err := s.sessionRepo.ListByUserID(userID)
	if err != nil {
		return nil, err
	}

	var vos []dto.SessionVO
	for _, s := range sessions {
		vos = append(vos, dto.SessionVO{
			ID:        s.ID,
			Title:     s.Title,
			CreatedAt: s.CreatedAt.Format(time.RFC3339),
		})
	}

	return vos, nil
}

func truncateTitle(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
