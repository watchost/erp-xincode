// Copyright 2026 zhouhouping. All Rights Reserved.

package repository

import (
	"gorm.io/gorm"
	"erp-system/internal/llm/model"
)

type SessionRepository interface {
	Create(session *model.LlmSession) error
	FindByID(id int64) (*model.LlmSession, error)
	ListByUserID(userID int64) ([]model.LlmSession, error)
}

type MessageRepository interface {
	Create(message *model.LlmMessage) error
	ListBySessionID(sessionID int64) ([]model.LlmMessage, error)
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
