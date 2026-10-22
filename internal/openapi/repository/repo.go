// Copyright 2026 zhouhouping. All Rights Reserved.

package repository

import (
	"erp-system/internal/openapi/model"
	"gorm.io/gorm"
)

type ClientRepository interface {
	FindByClientID(clientID string) (*model.OpenClient, error)
	Create(client *model.OpenClient) error
	Update(client *model.OpenClient) error
	List(page, pageSize int) ([]model.OpenClient, int64, error)
}

type TokenRepository interface {
	Create(token *model.OpenToken) error
	FindByAccessToken(accessToken string) (*model.OpenToken, error)
	FindByRefreshToken(refreshToken string) (*model.OpenToken, error)
	DeleteByAccessToken(accessToken string) error
}

type WebhookRepository interface {
	Create(webhook *model.OpenWebhook) error
	FindByID(id int64) (*model.OpenWebhook, error)
	ListByClientID(clientID string) ([]model.OpenWebhook, error)
	ListByEvent(event string) ([]model.OpenWebhook, error)
}

// WebhookDeliveryRepository 投递记录（worker 取待投递、推进状态）。
type WebhookDeliveryRepository interface {
	Create(d *model.OpenWebhookDelivery) error
	Update(d *model.OpenWebhookDelivery) error
	// ListDuePending 取 status=0 且 next_retry_at<=now 的投递，按创建时间升序。
	ListDuePending(limit int) ([]model.OpenWebhookDelivery, error)
	ListByWebhook(webhookID int64, page, pageSize int) ([]model.OpenWebhookDelivery, int64, error)
}

type SyncLogRepository interface {
	Create(log *model.OpenSyncLog) error
	FindByID(id int64) (*model.OpenSyncLog, error)
}

type clientRepo struct {
	db *gorm.DB
}

func NewClientRepository(db *gorm.DB) ClientRepository {
	return &clientRepo{db: db}
}

func (r *clientRepo) FindByClientID(clientID string) (*model.OpenClient, error) {
	var client model.OpenClient
	if err := r.db.Where("client_id = ?", clientID).First(&client).Error; err != nil {
		return nil, err
	}
	return &client, nil
}

func (r *clientRepo) Create(client *model.OpenClient) error {
	return r.db.Create(client).Error
}

func (r *clientRepo) Update(client *model.OpenClient) error {
	return r.db.Save(client).Error
}

func (r *clientRepo) List(page, pageSize int) ([]model.OpenClient, int64, error) {
	var clients []model.OpenClient
	var total int64
	if err := r.db.Model(&model.OpenClient{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := r.db.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&clients).Error; err != nil {
		return nil, 0, err
	}
	return clients, total, nil
}

type tokenRepo struct {
	db *gorm.DB
}

func NewTokenRepository(db *gorm.DB) TokenRepository {
	return &tokenRepo{db: db}
}

func (r *tokenRepo) Create(token *model.OpenToken) error {
	return r.db.Create(token).Error
}

func (r *tokenRepo) FindByAccessToken(accessToken string) (*model.OpenToken, error) {
	var token model.OpenToken
	if err := r.db.Where("access_token = ?", accessToken).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *tokenRepo) FindByRefreshToken(refreshToken string) (*model.OpenToken, error) {
	var token model.OpenToken
	if err := r.db.Where("refresh_token = ?", refreshToken).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *tokenRepo) DeleteByAccessToken(accessToken string) error {
	return r.db.Where("access_token = ?", accessToken).Delete(&model.OpenToken{}).Error
}

type webhookRepo struct {
	db *gorm.DB
}

func NewWebhookRepository(db *gorm.DB) WebhookRepository {
	return &webhookRepo{db: db}
}

func (r *webhookRepo) Create(webhook *model.OpenWebhook) error {
	return r.db.Create(webhook).Error
}

func (r *webhookRepo) FindByID(id int64) (*model.OpenWebhook, error) {
	var webhook model.OpenWebhook
	if err := r.db.Where("id = ?", id).First(&webhook).Error; err != nil {
		return nil, err
	}
	return &webhook, nil
}

func (r *webhookRepo) ListByClientID(clientID string) ([]model.OpenWebhook, error) {
	var webhooks []model.OpenWebhook
	if err := r.db.Where("client_id = ?", clientID).Find(&webhooks).Error; err != nil {
		return nil, err
	}
	return webhooks, nil
}

func (r *webhookRepo) ListByEvent(event string) ([]model.OpenWebhook, error) {
	var webhooks []model.OpenWebhook
	if err := r.db.Where("event = ? AND status = 1", event).Find(&webhooks).Error; err != nil {
		return nil, err
	}
	return webhooks, nil
}

type webhookDeliveryRepo struct {
	db *gorm.DB
}

func NewWebhookDeliveryRepository(db *gorm.DB) WebhookDeliveryRepository {
	return &webhookDeliveryRepo{db: db}
}

func (r *webhookDeliveryRepo) Create(d *model.OpenWebhookDelivery) error {
	return r.db.Create(d).Error
}

func (r *webhookDeliveryRepo) Update(d *model.OpenWebhookDelivery) error {
	return r.db.Save(d).Error
}

func (r *webhookDeliveryRepo) ListDuePending(limit int) ([]model.OpenWebhookDelivery, error) {
	var list []model.OpenWebhookDelivery
	err := r.db.Where("status = ? AND next_retry_at <= now()", model.DeliveryStatusPending).
		Order("created_at ASC").Limit(limit).Find(&list).Error
	return list, err
}

func (r *webhookDeliveryRepo) ListByWebhook(webhookID int64, page, pageSize int) ([]model.OpenWebhookDelivery, int64, error) {
	var list []model.OpenWebhookDelivery
	var total int64
	query := r.db.Model(&model.OpenWebhookDelivery{}).Where("webhook_id = ?", webhookID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

type syncLogRepo struct {
	db *gorm.DB
}

func NewSyncLogRepository(db *gorm.DB) SyncLogRepository {
	return &syncLogRepo{db: db}
}

func (r *syncLogRepo) Create(log *model.OpenSyncLog) error {
	return r.db.Create(log).Error
}

func (r *syncLogRepo) FindByID(id int64) (*model.OpenSyncLog, error) {
	var log model.OpenSyncLog
	if err := r.db.Where("id = ?", id).First(&log).Error; err != nil {
		return nil, err
	}
	return &log, nil
}
