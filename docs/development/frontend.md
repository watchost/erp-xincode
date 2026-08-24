# 前端开发任务

## 1. P0 整改（必须先做）

### 1.1 认证与权限
- 修复 `request.js` 响应拦截器：HTTP 401 清除 localStorage 跳登录；
- 实现 `v-permission` 自定义指令：
  ```js
  app.directive('permission', {
    mounted(el, binding) {
      const perms = userStore.permissions
      if (!perms.includes(binding.value)) el.parentNode?.removeChild(el)
    }
  })
  ```
- 路由加 `meta.permissions`，守卫校验；
- 菜单按权限动态渲染；
- 登录后调用 `getPermissions()` 存入 Pinia + localStorage。

### 1.2 删除演示代码
- 删除登录页硬编码 `admin/admin123`；
- 删除所有 view 的 catch 块中 `ElMessage.success('创建成功（模拟）')` 和 mock 数据；
- 删除 dashboard 的硬编码 KPI。

### 1.3 表单校验
- 所有 el-form 加 ref/rules/prop，提交前 validate；
- 数量输入 `:min="0.001"`；
- 新建用户密码加复杂度规则。

### 1.4 Token
- 拦截器处理 401；
- 实现 refresh token 逻辑（401 时尝试刷新 + 请求队列）。

## 2. P1 补全

### 2.1 新增页面
- 审计日志页 `/iam/audit-logs`（用户、模块、时间筛选）；
- 部门管理 `/iam/departments`（树）；
- 角色权限分配（穿梭框/树勾选）；
- 会计科目管理 `/finance/accounts`；
- 凭证管理 `/finance/vouchers`（手工录入、过账）；
- 生产入库报工 `/production/receipt`；
- BOM 树形管理 `/production/boms`（支持树形表格编辑）；
- 设备管理增强：在线状态、指令下发、消息流水；
- LLM 会话：多轮对话 UI、流式输出（SSE）；
- OpenAPI 客户端管理。

### 2.2 仪表板
- 真实 KPI 卡片；
- ECharts 趋势图（采购/销售按月）；
- LLM 分析面板（Markdown 渲染，注意用文本插值或安全的 markdown 渲染器）；
- onBeforeUnmount 销毁 chart 实例 + removeEventListener。

## 3. P2 新业务模块页面

### 3.1 销售模块
- `/sales/orders` 销售订单（明细表格、价格计算）；
- `/sales/outbound` 销售出库扫码；
- `/sales/returns` 销售退货；
- `/mdm/customers` 客户管理。

### 3.2 库存增强
- `/warehouse/stock-checks` 盘点（创建、录入实盘、差异、审批）；
- `/warehouse/transfers` 调拨；
- `/warehouse/batches` 批次列表；
- `/warehouse/serials` 序列号追溯（时间线）；
- `/warehouse/batch-trace` 批次追溯图。

### 3.3 财务增强
- `/finance/receivables` 应收 + `/finance/receipts` 收款；
- `/finance/payables` 应付 + `/finance/payments` 付款；
- `/finance/settlements` 核销；
- 资产负债表、利润表、现金流量表；
- 账龄分析表。

### 3.4 导入导出与打印
- 列表页加"导出 Excel"按钮；
- 主数据加"导入"（上传 + 校验 + 预览 + 确认）；
- 单据详情加"打印"按钮（弹出打印模板）。

## 4. P3 平台化

### 4.1 审批工作流
- 流程设计器（bpmn-js 或简化版拖拽节点）；
- 我的待办/已办；
- 审批历史时间线；
- 单据页加审批面板（通过/驳回/转办/加签）。

### 4.2 多租户
- 平台管理员租户管理页；
- 租户切换器（若用户属多租户）；
- 租户级配置（LOGO、名称）。

### 4.3 消息通知
- 顶部铃铛 + 未读数轮询；
- 消息中心（已读/未读、分类筛选）。

### 4.4 监控（可选）
- 内嵌 Grafana iframe。

### 4.5 PWA/移动端
- manifest.json + service worker；
- PDA 专用页面：大字体、大按钮、调用摄像头扫码（`html5-qrcode`）、离线队列。

## 5. 前端技术规范
- 金额展示统一 filter：`formatMoney(v) => Number(v).toLocaleString('zh-CN', {minimumFractionDigits:2})`；
- 金额计算用 decimal.js，禁止原生浮点；
- 全局 errorHandler + unhandledrejection 上报；
- 安全读取 localStorage（try-catch）；
- ECharts 实例必须 dispose；
- 禁止 v-html（LLM/富文本内容用 DOMPurify 净化后再渲染）；
- nginx 加安全头（见审计 M-1）；
- 静态资源缓存：`/assets/* expires 1y`，`index.html no-cache`。
