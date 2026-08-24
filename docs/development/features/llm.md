# LLM 智能分析

## 1. 目标
真正对接通义千问/文心一言，支持多轮对话、仪表分析、Prompt 模板管理；会话归属严格隔离。

## 2. 数据模型
见 [schema.md P1-3](../schema.md#p1-3-llm)。
- `llm_session` 增加 tenant_id/updated_at/deleted_at；
- `llm_message` 保持；
- 新增 `llm_prompt_template`。

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | /llm/chat | 对话（支持 SSE 流式） |
| GET | /llm/sessions | 我的会话列表 |
| POST | /llm/sessions | 新建会话 |
| DELETE | /llm/sessions/:id | 删除会话（仅本人） |
| GET | /llm/sessions/:id/history | 会话历史（校验归属） |
| POST | /dashboard/llm/analysis | 仪表智能分析（传当前 KPI） |
| GET/POST | /admin/llm/templates | Prompt 模板管理 |

## 4. 关键逻辑

### 4.1 网关
- 定义统一接口：
  ```go
  type Gateway interface {
      Chat(ctx context.Context, messages []Message, opts ChatOpts) (string, error)
      StreamChat(ctx context.Context, messages []Message, opts ChatOpts) (<-chan Chunk, error)
  }
  ```
- 实现 QwenGateway（真实调用 DashScope）、WenXinGateway（真实调用文心），或删除未实现的；
- API Key 从 `viper.GetString("llm.api_key")` 读取，启动时非空校验；
- 超时和限流（`llm.rate_limit`，用 Redis 令牌桶）。

### 4.2 多轮对话
- `buildMessages` 加载该会话最近 N 条（如 20 条）历史作为上下文；
- system prompt 根据场景（普通问答/仪表分析）加载模板；
- 新消息写入 llm_message 后再调用网关；
- 流式响应通过 SSE 推到前端，结束后保存完整 assistant 消息。

### 4.3 会话归属（修复 IDOR）
- 所有查询加 `WHERE user_id = ? AND tenant_id = ?`；
- 从 context 取 user_id（JWT 中间件设置，不能再 fallback 到 1）；
- 会话 ID 不可遍历敏感信息（用 UUID 而非自增 ID 暴露给前端，或保持自增但严格校验归属）。

### 4.4 仪表分析
- 后端拉取真实 KPI（采购额、销售额、库存价值、库存预警数）；
- 渲染到 prompt 模板，让 LLM 返回结构化的趋势分析和建议；
- 用 JSON mode（若网关支持）解析，降级为纯文本。

## 5. 安全/合规
- 敏感财务数据是否发第三方 LLM 需租户级配置（默认关闭财务字段）；
- Prompt 注入：系统 prompt 明确指令边界，用户输入不解析为指令；
- API Key 不进前端；
- 对话内容保留期限（可配置）。

## 6. 边界条件
- API Key 未配置：返回明确错误，不能假装成功；
- 上游超时/限流：返回友好提示，支持重试；
- 会话上下文超长：滑动窗口或摘要压缩。
