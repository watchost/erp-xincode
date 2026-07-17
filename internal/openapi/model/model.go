// Copyright 2026 zhouhouping. All Rights Reserved.

package model

import (
	"time"
)

type OpenClient struct {
	ID           int64     `json:"id"`
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"-"`
	Name         string    `json:"name"`
	Status       int       `json:"status"`
	Scopes       []byte    `json:"scopes"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type OpenToken struct {
	ID           int64     `json:"id"`
	ClientID     string    `json:"client_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type OpenWebhook struct {
	ID          int64     `json:"id"`
	ClientID    string    `json:"client_id"`
	Event       string    `json:"event"`
	URL         string    `json:"url"`
	Secret      string    `json:"secret"`
	Status      int       `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type OpenSyncLog struct {
	ID         int64     `json:"id"`
	ClientID   string    `json:"client_id"`
	SyncType   string    `json:"sync_type"`
	SyncData   []byte    `json:"sync_data"`
	Status     int       `json:"status"`
	ErrorMsg   string    `json:"error_msg"`
	CreatedAt  time.Time `json:"created_at"`
}

func (OpenClient) TableName() string { return "open_client" }
func (OpenToken) TableName() string { return "open_token" }
func (OpenWebhook) TableName() string { return "open_webhook" }
func (OpenSyncLog) TableName() string { return "open_sync_log" }
