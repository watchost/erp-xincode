# 文档索引

## 当前进度（2026-08-14）

P0 整改已完成第一批后端主链路修复：

- ✅ 真实 JWT 认证、路由权限校验、登录态上下文、登出/改密；
- ✅ HTTP 真实状态码、TraceID、Recovery、CORS、超时、优雅关闭；
- ✅ 库存移动加权平均成本、仓库/库位必填、行锁、原子 upsert；
- ✅ 采购入库与生产领料在单事务内完成库存、台账、单据数量/状态更新；
- ✅ 业务单号改为 Redis 日流水号，扫码接口接入幂等键；
- ✅ 生产模型与数据库结构通过 `0004_production_schema` 对齐。

仍未完成的 P0/P1 重点：JWT jti 黑名单接入中间件、密钥彻底外置与历史轮换、财务最小闭环、金额 decimal 化、自动化测试、健康检查/非 root 镜像、前端权限与错误处理改造。

## 代码审计（2026-08-13）

- [审计总报告 AUDIT.md](AUDIT.md) —— 合并去重后的结论、P0/P1 清单与修复路线图
- [01 · 后端安全与鉴权](audit/01-security.md)
- [02 · 业务逻辑与正确性](audit/02-business.md)
- [03 · 基础设施/配置/可观测性/代码质量](audit/03-infrastructure.md)
- [04 · 前端安全与逻辑](audit/04-frontend.md)

## 后续开发

- [开发总纲 DEVELOPMENT.md](DEVELOPMENT.md) —— 4 阶段 Roadmap（P0 整改 → P1 补全 → P2 扩展 → P3 平台化），含时间估算与验收清单
- 开发约定
  - [00 · 开发约定](development/00-conventions.md) —— 目录分层、API、事务、金额、错误处理、前端规范
  - [数据库变更](development/schema.md) —— 所有新表/新字段 DDL
- 功能设计
  - [IAM 用户/角色/权限/审计](development/features/iam.md)
  - [销售管理](development/features/sales.md)
  - [库存：盘点/调拨/批次](development/features/inventory.md)
  - [财务：凭证/科目/应收应付/预算](development/features/finance.md)
  - [生产：BOM/工单/成本卷积](development/features/production.md)
  - [设备 WebSocket/适配器](development/features/device.md)
  - [LLM 智能分析](development/features/llm.md)
  - [OpenAPI 第三方对接](development/features/openapi.md)
  - [审批工作流](development/features/workflow.md)
  - [多租户](development/features/tenant.md)
- [API 总览](development/api.md)
- [前端任务](development/frontend.md)

## 结论速读

审计初始版本发现系统**不可上线、不可投入真实业务数据**，共 8 项 P0 致命问题。第一批后端整改已消除认证空壳、权限空转、库存成本归零、跨服务事务拆分、并发无锁、单号/幂等缺失等主链路风险的后端基础问题；财务、前端、测试、部署安全和完整生产闭环仍需继续按 DEVELOPMENT.md 推进。

在剩余 P0 项验收完成前，仍不要连接包含真实业务/财务数据的数据库，也不要暴露到非可信网络。
