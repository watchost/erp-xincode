// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"erp-system/internal/openapi/dto"
	"erp-system/internal/openapi/model"
	"erp-system/internal/openapi/repository"
	"erp-system/internal/pkg/errors"
)

type OpenAPIService struct {
	clientRepo   repository.ClientRepository
	tokenRepo    repository.TokenRepository
	webhookRepo  repository.WebhookRepository
	syncLogRepo  repository.SyncLogRepository
}

func NewOpenAPIService(
	clientRepo repository.ClientRepository,
	tokenRepo repository.TokenRepository,
	webhookRepo repository.WebhookRepository,
	syncLogRepo repository.SyncLogRepository,
) *OpenAPIService {
	return &OpenAPIService{
		clientRepo:  clientRepo,
		tokenRepo:   tokenRepo,
		webhookRepo: webhookRepo,
		syncLogRepo: syncLogRepo,
	}
}

func (s *OpenAPIService) GetToken(ctx context.Context, req dto.TokenReq) (*dto.TokenRes, error) {
	client, err := s.clientRepo.FindByClientID(req.ClientID)
	if err != nil {
		return nil, errors.New(70001, 401, "无效的客户端凭证")
	}

	if client.ClientSecret != req.ClientSecret {
		return nil, errors.New(70002, 401, "客户端密钥错误")
	}

	if client.Status != 1 {
		return nil, errors.New(70003, 403, "客户端已禁用")
	}

	accessToken := uuid.NewString()
	refreshToken := uuid.NewString()
	expiresIn := int64(7200)
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)

	token := &model.OpenToken{
		ClientID:     req.ClientID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}

	if err := s.tokenRepo.Create(token); err != nil {
		return nil, err
	}

	return &dto.TokenRes{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
		RefreshToken: refreshToken,
		Scope:        req.Scope,
	}, nil
}

func (s *OpenAPIService) RefreshToken(ctx context.Context, refreshToken string) (*dto.TokenRes, error) {
	token, err := s.tokenRepo.FindByRefreshToken(refreshToken)
	if err != nil {
		return nil, errors.New(70004, 401, "无效的刷新令牌")
	}

	newAccessToken := uuid.NewString()
	newRefreshToken := uuid.NewString()
	expiresIn := int64(7200)
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)

	newToken := &model.OpenToken{
		ClientID:     token.ClientID,
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		ExpiresAt:    expiresAt,
	}

	if err := s.tokenRepo.Create(newToken); err != nil {
		return nil, err
	}

	_ = s.tokenRepo.DeleteByAccessToken(token.AccessToken)

	return &dto.TokenRes{
		AccessToken:  newAccessToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
		RefreshToken: newRefreshToken,
	}, nil
}

func (s *OpenAPIService) ValidateAccessToken(accessToken string) (*model.OpenToken, error) {
	token, err := s.tokenRepo.FindByAccessToken(accessToken)
	if err != nil {
		return nil, errors.New(70005, 401, "无效的访问令牌")
	}

	if token.ExpiresAt.Before(time.Now()) {
		return nil, errors.New(70006, 401, "访问令牌已过期")
	}

	return token, nil
}

func (s *OpenAPIService) CreateWebhook(ctx context.Context, req dto.WebhookReq) (*dto.WebhookVO, error) {
	client, err := s.clientRepo.FindByClientID(req.ClientID)
	if err != nil {
		return nil, errors.New(70001, 401, "无效的客户端")
	}

	webhook := &model.OpenWebhook{
		ClientID: client.ClientID,
		Event:    req.Event,
		URL:      req.URL,
		Secret:   req.Secret,
		Status:   1,
	}

	if err := s.webhookRepo.Create(webhook); err != nil {
		return nil, err
	}

	return &dto.WebhookVO{
		ID:        webhook.ID,
		ClientID:  webhook.ClientID,
		Event:     webhook.Event,
		URL:       webhook.URL,
		Status:    webhook.Status,
		CreatedAt: webhook.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (s *OpenAPIService) Sync(ctx context.Context, req dto.SyncReq) (*dto.SyncRes, error) {
	_, err := s.clientRepo.FindByClientID(req.ClientID)
	if err != nil {
		return nil, errors.New(70001, 401, "无效的客户端")
	}

	syncLog := &model.OpenSyncLog{
		ClientID: req.ClientID,
		SyncType: req.SyncType,
		SyncData: []byte(req.SyncData),
		Status:   1,
	}

	if err := s.syncLogRepo.Create(syncLog); err != nil {
		return nil, err
	}

	return &dto.SyncRes{
		SyncID: syncLog.ID,
		Status: "success",
	}, nil
}
