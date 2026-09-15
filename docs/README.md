# 文档索引

## 当前进度（2026-09-15）

**P0 验收清单（DEVELOPMENT.md §1.7）已全部勾销，后端 P0 关口关闭。**

此前已完成：

- ✅ 真实 JWT 认证、路由权限校验、登录态上下文、登出（jti 黑名单中间件）/改密；
- ✅ HTTP 真实状态码、TraceID、Recovery、CORS、超时、优雅关闭；
- ✅ 库存移动加权平均成本、仓库/库位必填、行锁、原子 upsert；
- ✅ 采购入库与生产领料在单事务内完成库存、台账、单据数量/状态更新；
- ✅ 业务单号改为 Redis 日流水号（带进程内兜底），扫码接口接入幂等键；
- ✅ 生产/财务模型与数据库结构对齐；
- ✅ 财务最小闭环：采购入库同事务借贷双分录 + 成本卡，预算审批原子占用/取消释放/超额拒批。

本次新增（P0 验收收口）：

- ✅ **正式迁移工具链**：`golang-migrate` + `go:embed` 内嵌 SQL，`erp-migrate up|down|version|force` CLI；compose 废弃 initdb 挂载，改为一次性 `erp-migrate` 任务，密钥全部走 `.env`（见 `.env.example`），服务端支持 `MIGRATE_AUTO=true`；
- ✅ **集成测试骨架**（`internal/testsupport`）：真实 Postgres，按包自动建库+迁移+清表，`make test-it` 一键运行；无 DB 环境自动 Skip；
- ✅ 库存/采购/财务/生产/dashboard/decimalx 六包共 29 个测试：移动平均价、并发不超卖（120 并发扣 100 库存）、故障注入整事务回滚、超收/超领/超产回滚、预算占用/释放/超额、借贷不平衡拒绝、资产负债表平衡、幂等键去重、十进制精度专项（0.1+0.2、除不尽均价、4 位标度成本卡、BOM 损耗展开）；
- ✅ **金额/数量全链路 decimal 化**（2026-09-15）：后端零 `float64`，统一 `shopspring/decimal`；JSON 仍出裸 number（前端零改动）；舍入只发生在落库边界（金额 2 位/数量 3 位/单价成本 4 位）；凭证借贷改精确相等（差 1 分也拒绝，60004）；嵌套明细 `dive` 校验补齐（零负单价数量返回 10005）；迁移 `0007` 单价列加宽 `NUMERIC(18,4)`；
- ✅ `govulncheck ./...`：0 个影响本代码的漏洞（仅 1 个依赖库级、调用路径不可达的通告）；依赖升级（go-redis v9.22、jwt v5.2.2、x/crypto、x/text）；
- ✅ 健康检查与非 root 镜像（`/healthz` 实测 200，Dockerfile `USER app`）。

进入 P1 前仍挂账的技术债：财务科目主数据与凭证头/分录规范化、前端权限与错误处理（含删 mock/伪成功提示；前端金额运算需引入 decimal.js 对齐后端 decimal 契约）验收、生产环境密钥轮换方案。详见 [DEVELOPMENT.md](DEVELOPMENT.md) §2。

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

审计初始版本发现系统**不可上线、不可投入真实业务数据**，共 8 项 P0 致命问题。后端主链路风险（认证空壳、权限空转、库存成本归零、跨服务事务拆分、并发无锁、单号/幂等缺失、财务无闭环、无测试无迁移工具链）均已消除并有自动化测试守护；P0 验收清单（§1.7）已于 2026-09-15 全部勾销。后端 decimal 化已完成；前端改造（含 decimal.js 金额运算）与 P1 功能补全仍需按 DEVELOPMENT.md 推进。

在前端验收与生产部署加固（真实密钥/轮换、镜像 tag、CI）完成前，仍不要连接包含真实业务/财务数据的数据库，也不要暴露到非可信网络。
