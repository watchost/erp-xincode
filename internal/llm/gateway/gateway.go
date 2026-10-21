// Copyright 2026 zhouhouping. All Rights Reserved.

package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"erp-system/internal/llm/dto"
)

type LLMProvider string

const (
	ProviderQwen LLMProvider = "qwen"
)

// ChatStreamFunc 流式输出的增量回调（每次返回一段新增文本）。
type ChatStreamFunc func(chunk string) error

type LLMGateway interface {
	Chat(ctx context.Context, messages []Message) (string, error)
	// ChatStream 流式对话：增量文本经 emit 回调，返回完整答案。
	ChatStream(ctx context.Context, messages []Message, emit ChatStreamFunc) (string, error)
	GetProviderName() LLMProvider
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

const dashscopeURL = "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation"

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

func (g *QwenGateway) newRequest(ctx context.Context, messages []Message, stream bool) (*http.Request, error) {
	reqBody := map[string]interface{}{
		"model": g.model,
		"input": map[string]interface{}{
			"messages": messages,
		},
	}
	if stream {
		// 增量输出：每个 SSE 事件的 output.text 只含新增片段。
		reqBody["input"].(map[string]interface{})["incremental_output"] = true
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", dashscopeURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("X-DashScope-SSE", "enable")
		req.Header.Set("Accept", "text/event-stream")
	}
	return req, nil
}

// extractText 从 DashScope 响应 JSON 中取 output.text。
func extractText(body []byte) (string, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if output, ok := result["output"].(map[string]interface{}); ok {
		if text, ok := output["text"].(string); ok {
			return text, nil
		}
	}
	if msg, ok := result["message"].(string); ok {
		return "", fmt.Errorf("dashscope error: %s", msg)
	}
	return "", fmt.Errorf("failed to parse LLM response")
}

func (g *QwenGateway) Chat(ctx context.Context, messages []Message) (string, error) {
	req, err := g.newRequest(ctx, messages, false)
	if err != nil {
		return "", err
	}
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("dashscope http %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return extractText(body)
}

// ChatStream 以 SSE 方式调用 DashScope，逐段回调增量文本。
func (g *QwenGateway) ChatStream(ctx context.Context, messages []Message, emit ChatStreamFunc) (string, error) {
	req, err := g.newRequest(ctx, messages, true)
	if err != nil {
		return "", err
	}
	// 流式请求不受 60s client timeout 限制（长回答），由 ctx 控制生命周期。
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("dashscope http %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var full strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		chunk, err := extractText([]byte(payload))
		if err != nil {
			continue // 心跳/元信息帧
		}
		full.WriteString(chunk)
		if emit != nil {
			if err := emit(chunk); err != nil {
				return full.String(), err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return full.String(), err
	}
	return full.String(), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func NewLLMGateway(provider LLMProvider, config map[string]string) LLMGateway {
	return NewQwenGateway(config["api_key"], config["model"])
}

func BuildContextForAnalysis(ctx *dto.AnalysisContext) string {
	return fmt.Sprintf(`你是一个ERP系统的智能分析助手。用户角色：%s。
时间范围：%s。
业务数据：%v。
用户问题：%s

请基于以上信息给出专业的分析和建议。`, ctx.UserRole, ctx.TimeRange, ctx.BusinessData, ctx.UserQuestion)
}
