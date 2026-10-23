// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"erp-system/internal/openapi/dto"
	"erp-system/internal/openapi/model"
	"erp-system/internal/openapi/repository"
	"erp-system/internal/pkg/errors"
	"github.com/google/uuid"
)

// OpenAPI 模块错误码 700xx 段。
const (
	errCodeInvalidClient   = 70001
	errCodeBadSecret       = 70002
	errCodeClientDisabled  = 70003
	errCodeBadRefreshToken = 70004
	errCodeBadAccessToken  = 70005
	errCodeTokenExpired    = 70006
	errCodeScopeDenied     = 70007
	errCodeInvalidURL      = 70008
)

// 支持的 scope。
const (
	ScopeRead  = "read"
	ScopeWrite = "write"
)

const tokenTTLSeconds = int64(7200)

type OpenAPIService struct {
	clientRepo   repository.ClientRepository
	tokenRepo    repository.TokenRepository
	webhookRepo  repository.WebhookRepository
	deliveryRepo repository.WebhookDeliveryRepository
	syncLogRepo  repository.SyncLogRepository
}

func NewOpenAPIService(
	clientRepo repository.ClientRepository,
	tokenRepo repository.TokenRepository,
	webhookRepo repository.WebhookRepository,
	deliveryRepo repository.WebhookDeliveryRepository,
	syncLogRepo repository.SyncLogRepository,
) *OpenAPIService {
	return &OpenAPIService{
		clientRepo:   clientRepo,
		tokenRepo:    tokenRepo,
		webhookRepo:  webhookRepo,
		deliveryRepo: deliveryRepo,
		syncLogRepo:  syncLogRepo,
	}
}

// parseScopes 解析客户端 scopes（JSONB 数组）。
func parseScopes(raw []byte) []string {
	var scopes []string
	_ = json.Unmarshal(raw, &scopes)
	return scopes
}

// grantScopes 计算授权 scope：请求为空 → 客户端全集；否则须为客户端子集。
func grantScopes(clientScopes []string, requested string) (string, error) {
	allowed := make(map[string]bool, len(clientScopes))
	for _, s := range clientScopes {
		allowed[s] = true
	}
	if strings.TrimSpace(requested) == "" {
		return strings.Join(clientScopes, " "), nil
	}
	var granted []string
	for _, s := range strings.Fields(requested) {
		if !allowed[s] {
			return "", errors.New(errCodeScopeDenied, 403, "申请的 scope 超出客户端授权范围: "+s)
		}
		granted = append(granted, s)
	}
	return strings.Join(granted, " "), nil
}

func (s *OpenAPIService) GetToken(ctx context.Context, req dto.TokenReq) (*dto.TokenRes, error) {
	client, err := s.clientRepo.FindByClientID(req.ClientID)
	if err != nil {
		return nil, errors.New(errCodeInvalidClient, 401, "无效的客户端凭证")
	}

	// client_secret bcrypt 存储；CompareHashAndPassword 本身为常数时间比较，
	// 再经 subtle.ConstantTimeCompare 校验结果位，双重防时序侧信道。
	cmpErr := bcrypt.CompareHashAndPassword([]byte(client.ClientSecret), []byte(req.ClientSecret))
	ok := 0
	if cmpErr == nil {
		ok = 1
	}
	if subtle.ConstantTimeCompare([]byte{byte(ok)}, []byte{1}) != 1 {
		return nil, errors.New(errCodeBadSecret, 401, "客户端密钥错误")
	}

	if client.Status != 1 {
		return nil, errors.New(errCodeClientDisabled, 403, "客户端已禁用")
	}
	if !client.ExpiresAt.IsZero() && client.ExpiresAt.Before(time.Now()) {
		return nil, errors.New(errCodeClientDisabled, 403, "客户端已过期")
	}

	scope, err := grantScopes(parseScopes(client.Scopes), req.Scope)
	if err != nil {
		return nil, err
	}

	return s.issueToken(client.ClientID, scope)
}

// hashToken P1.5 H5：token 落库前 SHA-256（hex，64 字符，varchar(128) 列宽足够）。
// 明文 token 只在签发响应中返回，库中仅存哈希；所有按 token 的查询一律按哈希。
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *OpenAPIService) issueToken(clientID, scope string) (*dto.TokenRes, error) {
	expiresAt := time.Now().Add(time.Duration(tokenTTLSeconds) * time.Second)
	accessToken := uuid.NewString()
	refreshToken := uuid.NewString()
	token := &model.OpenToken{
		ClientID:     clientID,
		AccessToken:  hashToken(accessToken),  // P1.5 H5：哈希落库
		RefreshToken: hashToken(refreshToken), // P1.5 H5：哈希落库
		Scope:        scope,
		ExpiresAt:    expiresAt,
	}
	if err := s.tokenRepo.Create(token); err != nil {
		return nil, err
	}
	return &dto.TokenRes{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    tokenTTLSeconds,
		RefreshToken: refreshToken,
		Scope:        scope,
	}, nil
}

// RefreshToken 旋转（P1.5 H11）：单事务内先 DELETE 旧 refresh（哈希匹配且未过期，
// RETURNING）再插入新 token 行。行数为 0 即拒绝——并发换发只有一个能消费成功，
// 旧 refresh 重放因行已删除而失败，天然防重放；删除或插入失败整体回滚。
// 存量明文 token（H5 修复前签发）随 2h TTL 自然过期，查询只认哈希。
func (s *OpenAPIService) RefreshToken(ctx context.Context, refreshToken string) (*dto.TokenRes, error) {
	accessToken := uuid.NewString()
	newRefreshToken := uuid.NewString()
	newToken := &model.OpenToken{
		AccessToken:  hashToken(accessToken),
		RefreshToken: hashToken(newRefreshToken),
		ExpiresAt:    time.Now().Add(time.Duration(tokenTTLSeconds) * time.Second),
	}

	err := s.tokenRepo.RotateRefreshToken(hashToken(refreshToken), newToken, func(consumed *model.OpenToken) error {
		if consumed.Revoked != 0 {
			return errors.New(errCodeBadRefreshToken, 401, "无效的刷新令牌")
		}
		client, err := s.clientRepo.FindByClientID(consumed.ClientID)
		if err != nil || client.Status != 1 {
			return errors.New(errCodeClientDisabled, 403, "客户端已禁用")
		}
		newToken.ClientID = consumed.ClientID
		newToken.Scope = consumed.Scope
		return nil
	})
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(errCodeBadRefreshToken, 401, "无效的刷新令牌")
		}
		return nil, err
	}

	return &dto.TokenRes{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    tokenTTLSeconds,
		RefreshToken: newRefreshToken,
		Scope:        newToken.Scope,
	}, nil
}

func (s *OpenAPIService) ValidateAccessToken(accessToken string) (*model.OpenToken, error) {
	// P1.5 H5：按哈希等值查询——哈希比较无时序侧信道价值，命中即视为令牌匹配。
	token, err := s.tokenRepo.FindByAccessTokenHash(hashToken(accessToken))
	if err != nil || token.Revoked != 0 {
		return nil, errors.New(errCodeBadAccessToken, 401, "无效的访问令牌")
	}

	if token.ExpiresAt.Before(time.Now()) {
		return nil, errors.New(errCodeTokenExpired, 401, "访问令牌已过期")
	}

	return token, nil
}

// HasScope 判断令牌是否持有指定 scope。
func HasScope(tokenScope, required string) bool {
	for _, s := range strings.Fields(tokenScope) {
		if s == required {
			return true
		}
	}
	return false
}

// ---------- 客户端管理（内部管理端） ----------

// CreateClient 生成 client_id/secret，secret 明文仅此一次返回，库中只存 bcrypt。
func (s *OpenAPIService) CreateClient(ctx context.Context, req dto.ClientCreateReq) (*dto.ClientCreateRes, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	secret := hex.EncodeToString(buf)
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	scopes := req.Scopes
	if len(scopes) == 0 {
		scopes = []string{ScopeRead}
	}
	scopesJSON, _ := json.Marshal(scopes)

	client := &model.OpenClient{
		ClientID:     "oc_" + hex.EncodeToString(buf[:8]),
		ClientSecret: string(hash),
		Name:         req.Name,
		Status:       1,
		Scopes:       scopesJSON,
	}
	if err := s.clientRepo.Create(client); err != nil {
		return nil, err
	}
	return &dto.ClientCreateRes{
		ClientID:     client.ClientID,
		ClientSecret: secret,
		Name:         client.Name,
		Scopes:       strings.Join(scopes, " "),
	}, nil
}

func (s *OpenAPIService) ListClients(ctx context.Context, page, pageSize int) ([]dto.ClientVO, int64, error) {
	clients, total, err := s.clientRepo.List(page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	vos := make([]dto.ClientVO, 0, len(clients))
	for _, c := range clients {
		vos = append(vos, dto.ClientVO{
			ID:        c.ID,
			ClientID:  c.ClientID,
			Name:      c.Name,
			Status:    c.Status,
			Scopes:    strings.Join(parseScopes(c.Scopes), " "),
			CreatedAt: c.CreatedAt.Format(time.RFC3339),
		})
	}
	return vos, total, nil
}

// ---------- Webhook ----------

// CreateWebhook clientID 来自 Bearer token（中间件注入），不接受 body 传入。
func (s *OpenAPIService) CreateWebhook(ctx context.Context, clientID string, req dto.WebhookReq) (*dto.WebhookVO, error) {
	if _, err := s.clientRepo.FindByClientID(clientID); err != nil {
		return nil, errors.New(errCodeInvalidClient, 401, "无效的客户端")
	}
	if err := ValidatePublicURL(req.URL); err != nil {
		return nil, errors.New(errCodeInvalidURL, 400, "webhook URL 不合法: "+err.Error())
	}

	webhook := &model.OpenWebhook{
		ClientID: clientID,
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

// DispatchEvent 业务事件入口：为订阅该事件的启用 webhook 生成待投递记录。
func (s *OpenAPIService) DispatchEvent(event string, payload []byte) (int, error) {
	hooks, err := s.webhookRepo.ListByEvent(event)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, h := range hooks {
		d := &model.OpenWebhookDelivery{
			WebhookID:   h.ID,
			Event:       event,
			Payload:     payload,
			Status:      model.DeliveryStatusPending,
			NextRetryAt: time.Now(),
		}
		if err := s.deliveryRepo.Create(d); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (s *OpenAPIService) Sync(ctx context.Context, clientID string, req dto.SyncReq) (*dto.SyncRes, error) {
	if _, err := s.clientRepo.FindByClientID(clientID); err != nil {
		return nil, errors.New(errCodeInvalidClient, 401, "无效的客户端")
	}

	syncLog := &model.OpenSyncLog{
		ClientID: clientID,
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
