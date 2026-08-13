# 文档索引

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

审计发现当前代码**不可上线、不可投入真实业务数据**，共 8 项 P0 致命问题：

1. JWT 中间件是空壳，任意非空 `Authorization` 即可通过全部接口；
2. 路由零权限校验，前端三级权限完全空白；
3. 库存移动平均成本公式写死为 0，存货估值永久归零；
4. 采购入库/生产领料把库存写入与单据更新拆成两个独立事务，必现账实不符；
5. 库存/收货/领料读-改-写无行锁，并发必超卖/超收/超领；
6. 业务单号用秒级时间戳生成且扫码接口无幂等；
7. 生产/财务模块的 Go 模型与 SQL 迁移大面积不一致（缺表、列名不符），接口调用即 SQL 报错；
8. 财务凭证从不生成、预算从不校验、报表口径错误。

后续开发按 DEVELOPMENT.md 的 4 阶段推进，总计约 18 周达到可上线状态。**在 P0 整改完成前，不开发新功能。**
