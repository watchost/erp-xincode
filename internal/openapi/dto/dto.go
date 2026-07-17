// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

type TokenReq struct {
	GrantType    string `json:"grant_type" binding:"required"`
	ClientID     string `json:"client_id" binding:"required"`
	ClientSecret string `json:"client_secret" binding:"required"`
	Scope        string `json:"scope"`
}

type TokenRes struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

type WebhookReq struct {
	ClientID string `json:"client_id" binding:"required"`
	Event    string `json:"event" binding:"required"`
	URL      string `json:"url" binding:"required"`
	Secret   string `json:"secret"`
}

type WebhookVO struct {
	ID        int64  `json:"id"`
	ClientID  string `json:"client_id"`
	Event     string `json:"event"`
	URL       string `json:"url"`
	Status    int    `json:"status"`
	CreatedAt string `json:"created_at"`
}

type SyncReq struct {
	ClientID string `json:"client_id" binding:"required"`
	SyncType string `json:"sync_type" binding:"required"`
	SyncData string `json:"sync_data" binding:"required"`
}

type SyncRes struct {
	SyncID int64  `json:"sync_id"`
	Status string `json:"status"`
}

type ClientVO struct {
	ID        int64  `json:"id"`
	ClientID  string `json:"client_id"`
	Name      string `json:"name"`
	Status    int    `json:"status"`
	Scopes    string `json:"scopes"`
	CreatedAt string `json:"created_at"`
}
