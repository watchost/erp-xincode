// Copyright 2026 zhouhouping. All Rights Reserved.

package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"erp-system/internal/llm/dto"
)

type LLMProvider string

const (
	ProviderQwen    LLMProvider = "qwen"
	ProviderWenXin  LLMProvider = "wenxin"
	ProviderZhipu   LLMProvider = "zhipu"
)

type LLMGateway interface {
	Chat(ctx context.Context, messages []Message) (string, error)
	GetProviderName() LLMProvider
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type QwenGateway struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewQwenGateway(apiKey, model string) *QwenGateway {
	return &QwenGateway{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (g *QwenGateway) GetProviderName() LLMProvider {
	return ProviderQwen
}

func (g *QwenGateway) Chat(ctx context.Context, messages []Message) (string, error) {
	reqBody := map[string]interface{}{
		"model": g.model,
		"input": map[string]interface{}{
			"messages": messages,
		},
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if output, ok := result["output"].(map[string]interface{}); ok {
		if text, ok := output["text"].(string); ok {
			return text, nil
		}
	}

	return "", fmt.Errorf("failed to parse LLM response")
}

type WenXinGateway struct {
	apiKey     string
	secretKey  string
	httpClient *http.Client
}

func NewWenXinGateway(apiKey, secretKey string) *WenXinGateway {
	return &WenXinGateway{
		apiKey:    apiKey,
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (g *WenXinGateway) GetProviderName() LLMProvider {
	return ProviderWenXin
}

func (g *WenXinGateway) Chat(ctx context.Context, messages []Message) (string, error) {
	return "文心一言API暂未对接", nil
}

func NewLLMGateway(provider LLMProvider, config map[string]string) LLMGateway {
	switch provider {
	case ProviderQwen:
		return NewQwenGateway(config["api_key"], config["model"])
	case ProviderWenXin:
		return NewWenXinGateway(config["api_key"], config["secret_key"])
	default:
		return NewQwenGateway(config["api_key"], config["model"])
	}
}

func BuildContextForAnalysis(ctx *dto.AnalysisContext) string {
	return fmt.Sprintf(`你是一个ERP系统的智能分析助手。用户角色：%s。
时间范围：%s。
业务数据：%v。
用户问题：%s

请基于以上信息给出专业的分析和建议。`, ctx.UserRole, ctx.TimeRange, ctx.BusinessData, ctx.UserQuestion)
}
