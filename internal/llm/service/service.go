// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"strings"
	"time"

	"erp-system/internal/llm/dto"
	"erp-system/internal/llm/gateway"
	"erp-system/internal/llm/model"
	"erp-system/internal/llm/repository"
	"erp-system/internal/pkg/errors"
)

// LLM 模块错误码 602xx 段。
const (
	errCodeSessionNotFound = 60201
	errCodeTemplateExists  = 60202
	errCodeTemplateNotFound = 60203
)

// historyLimit 多轮对话携带的最近消息条数（含本轮问题）。
const historyLimit = 20

// defaultChatSystemPrompt 无启用模板时的兜底系统提示。
const defaultChatSystemPrompt = "你是ERP系统的智能分析助手，帮助用户进行业务数据分析和决策支持。回答基于事实，不要编造数据。"

const fallbackAnswer = "AI服务暂时不可用，请稍后重试"

type LLMService struct {
	sessionRepo repository.SessionRepository
	messageRepo repository.MessageRepository
	promptRepo  repository.PromptTemplateRepository
	llmGateway  gateway.LLMGateway
}

func NewLLMService(
	sessionRepo repository.SessionRepository,
	messageRepo repository.MessageRepository,
	promptRepo repository.PromptTemplateRepository,
	llmGateway gateway.LLMGateway,
) *LLMService {
	return &LLMService{
		sessionRepo: sessionRepo,
		messageRepo: messageRepo,
		promptRepo:  promptRepo,
		llmGateway:  llmGateway,
	}
}

// resolveSession 校验会话归属（IDOR 防护）；sessionID=0 时新建会话。
func (s *LLMService) resolveSession(userID, sessionID int64, question string) (int64, error) {
	if sessionID == 0 {
		session := &model.LlmSession{
			UserID: userID,
			Title:  truncateTitle(question, 50),
		}
		if err := s.sessionRepo.Create(session); err != nil {
			return 0, err
		}
		return session.ID, nil
	}
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return 0, errors.New(errCodeSessionNotFound, 404, "会话不存在")
	}
	if session.UserID != userID {
		return 0, errors.New(10004, 403, "无权限访问")
	}
	return sessionID, nil
}

func (s *LLMService) Chat(ctx context.Context, userID int64, req dto.ChatReq) (*dto.ChatRes, error) {
	sessionID, messages, err := s.prepareChat(userID, req)
	if err != nil {
		return nil, err
	}

	answer, err := s.llmGateway.Chat(ctx, messages)
	if err != nil {
		answer = fallbackAnswer
	}

	if err := s.persistAnswer(sessionID, answer); err != nil {
		return nil, err
	}

	return &dto.ChatRes{
		SessionID: sessionID,
		Answer:    answer,
		Intent:    "analysis",
	}, nil
}

// ChatStream 流式对话：增量文本经 emit 回调，结束后落库完整答案。
func (s *LLMService) ChatStream(ctx context.Context, userID int64, req dto.ChatReq, emit gateway.ChatStreamFunc) (int64, error) {
	sessionID, messages, err := s.prepareChat(userID, req)
	if err != nil {
		return 0, err
	}

	answer, err := s.llmGateway.ChatStream(ctx, messages, emit)
	if err != nil {
		answer = fallbackAnswer
		if emit != nil && answer != "" {
			_ = emit(answer)
		}
	}

	if err := s.persistAnswer(sessionID, answer); err != nil {
		return 0, err
	}
	return sessionID, nil
}

// prepareChat 公共前置：解析/校验会话、落库用户消息、拼装多轮上下文。
func (s *LLMService) prepareChat(userID int64, req dto.ChatReq) (int64, []gateway.Message, error) {
	sessionID, err := s.resolveSession(userID, req.SessionID, req.Question)
	if err != nil {
		return 0, nil, err
	}

	userMessage := &model.LlmMessage{
		SessionID: sessionID,
		Role:      model.RoleUser,
		Content:   req.Question,
	}
	if err := s.messageRepo.Create(userMessage); err != nil {
		return 0, nil, err
	}

	messages, err := s.buildMessages(sessionID)
	if err != nil {
		return 0, nil, err
	}
	return sessionID, messages, nil
}

func (s *LLMService) persistAnswer(sessionID int64, answer string) error {
	return s.messageRepo.Create(&model.LlmMessage{
		SessionID: sessionID,
		Role:      model.RoleAssistant,
		Content:   answer,
	})
}

// buildMessages 多轮对话：系统提示（Prompt 模板）+ 最近 N 条历史。
func (s *LLMService) buildMessages(sessionID int64) ([]gateway.Message, error) {
	systemPrompt := defaultChatSystemPrompt
	if s.promptRepo != nil {
		if tpl, err := s.promptRepo.FindEnabledByScene(model.PromptSceneChat); err == nil && tpl != nil {
			systemPrompt = tpl.Content
		}
	}

	messages := []gateway.Message{{Role: "system", Content: systemPrompt}}

	history, err := s.messageRepo.ListRecentBySessionID(sessionID, historyLimit)
	if err != nil {
		return nil, err
	}
	for _, m := range history {
		switch m.Role {
		case model.RoleUser:
			messages = append(messages, gateway.Message{Role: "user", Content: m.Content})
		case model.RoleAssistant:
			messages = append(messages, gateway.Message{Role: "assistant", Content: m.Content})
		}
	}
	return messages, nil
}

// GetSessionHistory 校验会话归属后返回历史（IDOR 防护）。
func (s *LLMService) GetSessionHistory(ctx context.Context, userID, sessionID int64) ([]dto.MessageVO, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, errors.New(errCodeSessionNotFound, 404, "会话不存在")
	}
	if session.UserID != userID {
		return nil, errors.New(10004, 403, "无权限访问")
	}

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

// ---------- Prompt 模板管理 ----------

func (s *LLMService) CreatePromptTemplate(ctx context.Context, req dto.PromptTemplateReq) (*dto.PromptTemplateVO, error) {
	if existing, _ := s.promptRepo.FindByCode(req.Code); existing != nil {
		return nil, errors.New(errCodeTemplateExists, 409, "模板编码已存在")
	}
	tpl := &model.LlmPromptTemplate{
		Code:    req.Code,
		Name:    req.Name,
		Scene:   req.Scene,
		Content: req.Content,
		Enabled: 1,
	}
	if err := s.promptRepo.Create(tpl); err != nil {
		return nil, err
	}
	return promptTemplateVO(tpl), nil
}

func (s *LLMService) UpdatePromptTemplate(ctx context.Context, id int64, req dto.PromptTemplateReq) (*dto.PromptTemplateVO, error) {
	tpl, err := s.promptRepo.FindByID(id)
	if err != nil {
		return nil, errors.New(errCodeTemplateNotFound, 404, "模板不存在")
	}
	if req.Code != tpl.Code {
		if existing, _ := s.promptRepo.FindByCode(req.Code); existing != nil {
			return nil, errors.New(errCodeTemplateExists, 409, "模板编码已存在")
		}
	}
	tpl.Code = req.Code
	tpl.Name = req.Name
	tpl.Scene = req.Scene
	tpl.Content = req.Content
	if req.Enabled != nil {
		tpl.Enabled = *req.Enabled
	}
	if err := s.promptRepo.Update(tpl); err != nil {
		return nil, err
	}
	return promptTemplateVO(tpl), nil
}

func (s *LLMService) DeletePromptTemplate(ctx context.Context, id int64) error {
	if _, err := s.promptRepo.FindByID(id); err != nil {
		return errors.New(errCodeTemplateNotFound, 404, "模板不存在")
	}
	return s.promptRepo.Delete(id)
}

func (s *LLMService) ListPromptTemplates(ctx context.Context, scene string, page, pageSize int) ([]dto.PromptTemplateVO, int64, error) {
	list, total, err := s.promptRepo.List(scene, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	vos := make([]dto.PromptTemplateVO, 0, len(list))
	for _, t := range list {
		vos = append(vos, *promptTemplateVO(&t))
	}
	return vos, total, nil
}

func promptTemplateVO(t *model.LlmPromptTemplate) *dto.PromptTemplateVO {
	return &dto.PromptTemplateVO{
		ID:        t.ID,
		Code:      t.Code,
		Name:      t.Name,
		Scene:     t.Scene,
		Content:   t.Content,
		Enabled:   t.Enabled,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
	}
}

// truncateTitle 生成会话标题，按 rune 保留前 maxLen 个字符。
// P1.5 H12：原实现 s[:maxLen] 按字节截断，会切断多字节 UTF-8 字符（中文 3 字节），
// 非法字节序列写入 PG 报 invalid byte sequence，长中文提问建会话直接 500。
// 先用 strings.ToValidUTF8 剔除输入中已有的非法字节兜底，再按 rune 截断。
func truncateTitle(s string, maxLen int) string {
	s = strings.ToValidUTF8(s, "")
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
