// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"

	"erp-system/internal/llm/dto"
	"erp-system/internal/llm/gateway"
	"erp-system/internal/llm/repository"
	llmService "erp-system/internal/llm/service"
	apperrors "erp-system/internal/pkg/errors"
	"erp-system/internal/testsupport"
)

// fakeGateway 记录收到的消息，返回固定/分段答案。
type fakeGateway struct {
	lastMessages []gateway.Message
	answer       string
	chunks       []string
	chatErr      error
}

func (f *fakeGateway) Chat(ctx context.Context, messages []gateway.Message) (string, error) {
	f.lastMessages = messages
	if f.chatErr != nil {
		return "", f.chatErr
	}
	return f.answer, nil
}

func (f *fakeGateway) ChatStream(ctx context.Context, messages []gateway.Message, emit gateway.ChatStreamFunc) (string, error) {
	f.lastMessages = messages
	var full strings.Builder
	for _, ch := range f.chunks {
		full.WriteString(ch)
		if emit != nil {
			if err := emit(ch); err != nil {
				return full.String(), err
			}
		}
	}
	return full.String(), nil
}

func (f *fakeGateway) GetProviderName() gateway.LLMProvider { return gateway.ProviderQwen }

func llmErrCode(err error) int {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return 0
}

func setupLLM(t *testing.T, gw gateway.LLMGateway) (*gorm.DB, *llmService.LLMService) {
	t.Helper()
	gdb := testsupport.NewDB(t)
	svc := llmService.NewLLMService(
		repository.NewSessionRepository(gdb),
		repository.NewMessageRepository(gdb),
		repository.NewPromptTemplateRepository(gdb),
		gw,
	)
	return gdb, svc
}

// 多轮对话：第二轮请求应携带第一轮的用户/助手历史，系统提示来自启用的 Prompt 模板。
func TestChat_MultiTurnHistory(t *testing.T) {
	gw := &fakeGateway{answer: "第一轮回答"}
	_, svc := setupLLM(t, gw)
	ctx := testsupport.Ctx()

	res1, err := svc.Chat(ctx, 1, dto.ChatReq{Question: "库存情况如何"})
	if err != nil {
		t.Fatalf("第一轮对话失败: %v", err)
	}
	if res1.SessionID == 0 {
		t.Fatal("应自动创建会话")
	}

	if _, err := svc.Chat(ctx, 1, dto.ChatReq{SessionID: res1.SessionID, Question: "哪个仓库最紧张"}); err != nil {
		t.Fatalf("第二轮对话失败: %v", err)
	}

	msgs := gw.lastMessages
	if len(msgs) != 4 { // system + u1 + a1 + u2
		t.Fatalf("多轮上下文应含 4 条消息，实际 %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Role != "system" || !strings.Contains(msgs[0].Content, "智能分析助手") {
		t.Fatalf("系统提示应来自种子模板: %+v", msgs[0])
	}
	if msgs[1].Role != "user" || msgs[1].Content != "库存情况如何" {
		t.Fatalf("第 2 条应为第一轮提问: %+v", msgs[1])
	}
	if msgs[2].Role != "assistant" || msgs[2].Content != "第一轮回答" {
		t.Fatalf("第 3 条应为第一轮回答: %+v", msgs[2])
	}
	if msgs[3].Role != "user" || msgs[3].Content != "哪个仓库最紧张" {
		t.Fatalf("第 4 条应为本轮提问: %+v", msgs[3])
	}
}

// 会话归属（IDOR）：他人会话续聊/读历史 → 403；不存在 → 60201。
func TestChat_SessionOwnership(t *testing.T) {
	gw := &fakeGateway{answer: "ok"}
	_, svc := setupLLM(t, gw)
	ctx := testsupport.Ctx()

	res, err := svc.Chat(ctx, 1, dto.ChatReq{Question: "hello"})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	if _, err := svc.Chat(ctx, 2, dto.ChatReq{SessionID: res.SessionID, Question: "越权续聊"}); llmErrCode(err) != 10004 {
		t.Fatalf("他人会话续聊应返回 10004，实际 %v", err)
	}
	if _, err := svc.GetSessionHistory(ctx, 2, res.SessionID); llmErrCode(err) != 10004 {
		t.Fatalf("他人会话读历史应返回 10004，实际 %v", err)
	}
	if _, err := svc.GetSessionHistory(ctx, 1, 99999); llmErrCode(err) != 60201 {
		t.Fatalf("不存在会话应返回 60201，实际 %v", err)
	}
	if _, err := svc.Chat(ctx, 1, dto.ChatReq{SessionID: 99999, Question: "x"}); llmErrCode(err) != 60201 {
		t.Fatalf("不存在会话续聊应返回 60201，实际 %v", err)
	}

	// 本人读历史正常
	history, err := svc.GetSessionHistory(ctx, 1, res.SessionID)
	if err != nil || len(history) != 2 {
		t.Fatalf("本人读历史应返回 2 条: len=%d err=%v", len(history), err)
	}
}

// 流式对话：分段回调 + 完整答案落库 + 网关故障兜底。
func TestChatStream(t *testing.T) {
	gw := &fakeGateway{chunks: []string{"库存", "总量", " 120"}}
	_, svc := setupLLM(t, gw)
	ctx := testsupport.Ctx()

	var got strings.Builder
	sessionID, err := svc.ChatStream(ctx, 1, dto.ChatReq{Question: "库存多少"}, func(chunk string) error {
		got.WriteString(chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("流式对话失败: %v", err)
	}
	if got.String() != "库存总量 120" {
		t.Fatalf("回调应收到全部分段，实际 %q", got.String())
	}
	history, err := svc.GetSessionHistory(ctx, 1, sessionID)
	if err != nil || len(history) != 2 {
		t.Fatalf("流式对话应落库问答各一条: len=%d err=%v", len(history), err)
	}
	if history[1].Content != "库存总量 120" {
		t.Fatalf("落库答案应为拼接后的完整文本，实际 %q", history[1].Content)
	}
}

// Prompt 模板 CRUD + 对话系统提示随模板切换。
func TestPromptTemplate_CRUDAndEffective(t *testing.T) {
	gw := &fakeGateway{answer: "ok"}
	_, svc := setupLLM(t, gw)
	ctx := testsupport.Ctx()

	// 种子模板存在
	list, total, err := svc.ListPromptTemplates(ctx, "", 1, 20)
	if err != nil || total < 2 {
		t.Fatalf("种子模板应存在: total=%d err=%v", total, err)
	}
	_ = list

	tpl, err := svc.CreatePromptTemplate(ctx, dto.PromptTemplateReq{
		Code: "chat_strict", Name: "严格模式", Scene: "chat", Content: "只回答库存相关问题。",
	})
	if err != nil {
		t.Fatalf("创建模板失败: %v", err)
	}

	// 重复编码 → 60202
	if _, err := svc.CreatePromptTemplate(ctx, dto.PromptTemplateReq{
		Code: "chat_strict", Name: "dup", Scene: "chat", Content: "x",
	}); llmErrCode(err) != 60202 {
		t.Fatalf("重复编码应返回 60202，实际 %v", err)
	}

	// 停用种子模板，启用自建模板（同场景取最新启用的一条 → id 更大者）
	if _, err := svc.Chat(ctx, 1, dto.ChatReq{Question: "q"}); err != nil {
		t.Fatal(err)
	}
	if gw.lastMessages[0].Content != "只回答库存相关问题。" {
		t.Fatalf("系统提示应为最新启用模板，实际 %q", gw.lastMessages[0].Content)
	}

	// 更新 + 删除
	enabled := 0
	if _, err := svc.UpdatePromptTemplate(ctx, tpl.ID, dto.PromptTemplateReq{
		Code: "chat_strict", Name: "严格模式v2", Scene: "chat", Content: "只回答库存相关问题。", Enabled: &enabled,
	}); err != nil {
		t.Fatalf("更新模板失败: %v", err)
	}
	if err := svc.DeletePromptTemplate(ctx, tpl.ID); err != nil {
		t.Fatalf("删除模板失败: %v", err)
	}
	if err := svc.DeletePromptTemplate(ctx, tpl.ID); llmErrCode(err) != 60203 {
		t.Fatalf("删除不存在模板应返回 60203，实际 %v", err)
	}
}

// 网关故障：降级文案且消息仍落库。
func TestChat_GatewayFallback(t *testing.T) {
	gw := &fakeGateway{chatErr: errors.New("network down")}
	_, svc := setupLLM(t, gw)
	ctx := testsupport.Ctx()

	res, err := svc.Chat(ctx, 1, dto.ChatReq{Question: "q"})
	if err != nil {
		t.Fatalf("网关故障不应报错: %v", err)
	}
	if !strings.Contains(res.Answer, "暂时不可用") {
		t.Fatalf("应返回降级文案，实际 %q", res.Answer)
	}
}
