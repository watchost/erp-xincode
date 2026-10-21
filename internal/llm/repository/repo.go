// Copyright 2026 zhouhouping. All Rights Reserved.

package repository

import (
	"erp-system/internal/llm/model"
	"gorm.io/gorm"
)

type SessionRepository interface {
	Create(session *model.LlmSession) error
	FindByID(id int64) (*model.LlmSession, error)
	ListByUserID(userID int64) ([]model.LlmSession, error)
}

type MessageRepository interface {
	Create(message *model.LlmMessage) error
	ListBySessionID(sessionID int64) ([]model.LlmMessage, error)
	// ListRecentBySessionID 取最近 limit 条（按时间升序返回，供多轮对话拼上下文）。
	ListRecentBySessionID(sessionID int64, limit int) ([]model.LlmMessage, error)
}

// PromptTemplateRepository Prompt 模板。
type PromptTemplateRepository interface {
	Create(t *model.LlmPromptTemplate) error
	Update(t *model.LlmPromptTemplate) error
	FindByID(id int64) (*model.LlmPromptTemplate, error)
	FindByCode(code string) (*model.LlmPromptTemplate, error)
	// FindEnabledByScene 取场景下最新启用的模板；无则返回 (nil, nil)。
	FindEnabledByScene(scene string) (*model.LlmPromptTemplate, error)
	List(scene string, page, pageSize int) ([]model.LlmPromptTemplate, int64, error)
	Delete(id int64) error
}

type sessionRepo struct {
	db *gorm.DB
}

func NewSessionRepository(db *gorm.DB) SessionRepository {
	return &sessionRepo{db: db}
}

func (r *sessionRepo) Create(session *model.LlmSession) error {
	return r.db.Create(session).Error
}

func (r *sessionRepo) FindByID(id int64) (*model.LlmSession, error) {
	var session model.LlmSession
	if err := r.db.Where("id = ?", id).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *sessionRepo) ListByUserID(userID int64) ([]model.LlmSession, error) {
	var sessions []model.LlmSession
	if err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

type messageRepo struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) MessageRepository {
	return &messageRepo{db: db}
}

func (r *messageRepo) Create(message *model.LlmMessage) error {
	return r.db.Create(message).Error
}

func (r *messageRepo) ListBySessionID(sessionID int64) ([]model.LlmMessage, error) {
	var messages []model.LlmMessage
	if err := r.db.Where("session_id = ?", sessionID).Order("created_at ASC").Find(&messages).Error; err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *messageRepo) ListRecentBySessionID(sessionID int64, limit int) ([]model.LlmMessage, error) {
	var desc []model.LlmMessage
	if err := r.db.Where("session_id = ?", sessionID).Order("id DESC").Limit(limit).Find(&desc).Error; err != nil {
		return nil, err
	}
	// 反转为时间升序
	messages := make([]model.LlmMessage, 0, len(desc))
	for i := len(desc) - 1; i >= 0; i-- {
		messages = append(messages, desc[i])
	}
	return messages, nil
}

type promptTemplateRepo struct {
	db *gorm.DB
}

func NewPromptTemplateRepository(db *gorm.DB) PromptTemplateRepository {
	return &promptTemplateRepo{db: db}
}

func (r *promptTemplateRepo) Create(t *model.LlmPromptTemplate) error {
	return r.db.Create(t).Error
}

func (r *promptTemplateRepo) Update(t *model.LlmPromptTemplate) error {
	return r.db.Save(t).Error
}

func (r *promptTemplateRepo) FindByID(id int64) (*model.LlmPromptTemplate, error) {
	var t model.LlmPromptTemplate
	if err := r.db.Where("id = ?", id).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *promptTemplateRepo) FindByCode(code string) (*model.LlmPromptTemplate, error) {
	var t model.LlmPromptTemplate
	if err := r.db.Where("code = ?", code).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *promptTemplateRepo) FindEnabledByScene(scene string) (*model.LlmPromptTemplate, error) {
	var t model.LlmPromptTemplate
	err := r.db.Where("scene = ? AND enabled = 1", scene).Order("id DESC").First(&t).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (r *promptTemplateRepo) List(scene string, page, pageSize int) ([]model.LlmPromptTemplate, int64, error) {
	var list []model.LlmPromptTemplate
	var total int64
	query := r.db.Model(&model.LlmPromptTemplate{})
	if scene != "" {
		query = query.Where("scene = ?", scene)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *promptTemplateRepo) Delete(id int64) error {
	return r.db.Delete(&model.LlmPromptTemplate{}, id).Error
}
