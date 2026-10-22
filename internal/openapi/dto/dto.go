// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

import "github.com/shopspring/decimal"

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

// WebhookReq client_id 从 Bearer token 提取，不接受 body 传入。
type WebhookReq struct {
	Event  string `json:"event" binding:"required"`
	URL    string `json:"url" binding:"required"`
	Secret string `json:"secret"`
}

type WebhookVO struct {
	ID        int64  `json:"id"`
	ClientID  string `json:"client_id"`
	Event     string `json:"event"`
	URL       string `json:"url"`
	Status    int    `json:"status"`
	CreatedAt string `json:"created_at"`
}

// SyncReq client_id 从 Bearer token 提取。
type SyncReq struct {
	SyncType string `json:"sync_type" binding:"required"`
	SyncData string `json:"sync_data" binding:"required"`
}

// ClientCreateReq 管理端创建客户端。
type ClientCreateReq struct {
	Name   string   `json:"name" binding:"required"`
	Scopes []string `json:"scopes"`
}

// ClientCreateRes 创建结果：client_secret 明文仅此一次返回。
type ClientCreateRes struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Name         string `json:"name"`
	Scopes       string `json:"scopes"`
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

// ---------- 标准资源 API ----------

type MaterialVO struct {
	MaterialCode string `json:"material_code"`
	Name         string `json:"name"`
	Spec         string `json:"spec"`
	Unit         string `json:"unit"`
	Status       int    `json:"status"`
}

type InventoryVO struct {
	MaterialCode  string          `json:"material_code"`
	MaterialName  string          `json:"material_name"`
	WarehouseCode string          `json:"warehouse_code"`
	WarehouseName string          `json:"warehouse_name"`
	Qty           decimal.Decimal `json:"qty"`
	AvailableQty  decimal.Decimal `json:"available_qty"`
}

type OrderItemReq struct {
	MaterialCode string          `json:"material_code" binding:"required"`
	Qty          decimal.Decimal `json:"qty" binding:"required,decgt=0"`
	Price        decimal.Decimal `json:"price" binding:"decgte=0"`
}

type OrderCreateReq struct {
	SupplierCode string         `json:"supplier_code" binding:"required"`
	PlanArriveAt string         `json:"plan_arrive_at"`
	Items        []OrderItemReq `json:"items" binding:"required,min=1,dive"`
}

type OrderItemVO struct {
	MaterialCode string          `json:"material_code"`
	Qty          decimal.Decimal `json:"qty"`
	ReceivedQty  decimal.Decimal `json:"received_qty"`
	Price        decimal.Decimal `json:"price"`
}

type OrderVO struct {
	OrderNo      string          `json:"order_no"`
	SupplierCode string          `json:"supplier_code"`
	Status       int             `json:"status"`
	TotalAmount  decimal.Decimal `json:"total_amount"`
	CreatedAt    string          `json:"created_at"`
	Items        []OrderItemVO   `json:"items"`
}
