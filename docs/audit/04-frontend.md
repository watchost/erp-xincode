# 04 · 前端安全与逻辑审计

> 审计范围：`web/` 全部源码（28 个源文件）
> 技术栈：Vue 3.5.40 + Vite 5.4.21 + Element Plus 2.14.3 + Pinia 2.1.7 + Vue Router 4.3.0 + Axios 1.18.1

---

## 严重

### S-1 登录页硬编码默认账号密码并明文展示
- 文件：`web/src/views/login/index.vue:63-66`、`:45`

```js
const loginForm = reactive({
  username: 'admin',
  password: 'admin123'
})
```
```html
<p>默认账号：admin / admin123</p>
```

任何访问系统的人都能看到管理员凭据。若后端未强制改密，攻击者可直接登录获最高权限。

修复：删除硬编码，初始化为空字符串；删除模板提示；后端强制首登改密。

### S-2 完全缺失权限控制——任何登录用户可访问所有页面和功能
- 文件：`web/src/router/index.js:122-137`、`web/src/layouts/MainLayout.vue:16-86`

```js
router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('token')
  if (to.path === '/login') {
    if (token) next('/'); else next()
  } else {
    if (token) next(); else next('/login')   // 只检查 token 是否存在
  }
})
```

- 16 条路由全部静态定义，无 `meta.permission/meta.roles`；
- 菜单在 MainLayout 全部硬编码；
- 全项目搜索 `v-permission/v-auth/hasPermission` 零结果；`getPermissions` API 在 `api/iam.js:20` 定义但从未调用。

仓库管理员可直接敲 `/iam/users` 进入用户管理。OWASP A01:2021。

修复：路由 `meta.permissions`；守卫调用 `getPermissions()` 校验；菜单按权限动态渲染；`v-permission` 自定义指令做按钮级控制。

### S-3 Token 存储于 localStorage，存在 XSS 窃取风险
- 文件：`web/src/utils/request.js:14`、`web/src/store/user.js:8,19`、`web/src/router/index.js:123`

```js
config.headers['Authorization'] = 'Bearer ' + token
```

localStorage 可被同域任意 JS 读取。一旦有 XSS（LLM 返回、物料属性 JSONB、供应商名称等字段），token 直接被窃。无 httpOnly/Secure/SameSite。

修复：优先改为 `HttpOnly; Secure; SameSite=Strict` cookie；或配合严格 CSP、短 access token、refresh 轮换。

---

## 高

### H-1 HTTP 401 未清除登录态并跳转登录页
- 文件：`web/src/utils/request.js:25-42`

```js
(response) => {
  if (res.code !== 0) {
    if (res.code === 10001) {  // 仅业务码 10001 登出
      localStorage.removeItem('token'); router.push('/login')
    }
    return Promise.reject(...)
  }
},
(error) => {
  ElMessage.error(error.message || '网络错误')  // HTTP 401 不处理
  return Promise.reject(error)
}
```

token 过期时后端返回 HTTP 401，拦截器不检查 status，用户卡住反复看"网络错误"。

修复：
```js
if (error.response?.status === 401) {
  localStorage.removeItem('token'); localStorage.removeItem('userInfo')
  router.push('/login')
}
```

### H-2 所有创建/审批表单零校验
- 文件：`views/iam/users.vue:65-84`、`views/mdm/materials.vue:57-83`、`views/mdm/suppliers.vue:55-78`、`views/device/list.vue:74-103`、`views/production/work-orders.vue:68-87`、`views/purchase/orders.vue:83-140`

```html
<el-form :model="createForm" label-width="100px">  <!-- 无 :rules/ref/prop -->
```

`handleCreate` 直接 `createUser(createForm)` 提交，不 validate。必填项可空，手机号无格式，密码无复杂度。登录页是唯一有 rules 的表单。

修复：所有 el-form 加 ref/rules/prop，提交前 `formRef.value.validate()`。

### H-3 API 失败时 catch 块弹出"成功"提示并展示伪造数据
- 文件（几乎所有 view）：`views/iam/users.vue:151-155`、`views/mdm/materials.vue:167-171`、`views/purchase/orders.vue:249-253,268-270`、`views/production/work-orders.vue:169-173,188-190`、`views/device/list.vue:187-191`、`views/mdm/suppliers.vue:154-158`、`views/warehouse/inbound.vue:156-166`、`views/warehouse/outbound.vue:148-158`、`views/dashboard/index.vue:174-190`

```js
} catch (e) {
  ElMessage.success('创建成功（模拟）')   // API 失败却提示成功
  showCreateDialog.value = false
  loadData()
}
```

dashboard catch 中写入虚构销售数据：
```js
overviewData.value = { purchaseOrderCount: 42, salesAmount: 1258000, materialCount: 328, stockAlertCount: 15 }
```

ERP 场景下用户可能据此做出错误业务决策。

修复：删除 catch 中所有 success 和 mock 数据；失败显示 `ElMessage.error` 并保持对话框。

### H-4 无 Token 刷新/过期机制
全项目搜索 `refresh` 无结果。access_token 在 localStorage 中长期存在，守卫只检查存在性不检查过期。

修复：refresh token 机制（httpOnly cookie 存 refresh）；axios 401 拦截刷新+重试队列；或检查 JWT exp 主动刷新。

---

## 中

### M-1 缺少 Content-Security-Policy 及安全响应头
- 文件：`web/nginx.conf`

仅有 gzip 和 try_files，无任何 add_header。缺失 CSP、X-Frame-Options、X-Content-Type-Options、HSTS、Referrer-Policy、Permissions-Policy。结合 S-3，XSS 后无纵深防御。

修复：
```nginx
add_header X-Frame-Options "DENY" always;
add_header X-Content-Type-Options "nosniff" always;
add_header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self';" always;
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
```

### M-2 新建用户密码无强度要求
- 文件：`views/iam/users.vue:76-78`

无 rules，可设 1 位密码。修复：最少 8 位，大小写+数字。

### M-3 采购订单前端不计算 total_amount，金额展示无格式化
- 文件：`views/purchase/orders.vue:164-170,46-50`

`total_amount` 由后端计算（正确），但前端表单无明细金额汇总预览；展示 `¥{{ row.total_amount }}` 无千分位/精度。未发现前端浮点运算（因前端不计算金额）。

修复：明细列展示 `qty*price`（decimal.js 或整数分），底部合计；展示用 `toLocaleString('zh-CN',{minimumFractionDigits:2})`。

### M-4 localStorage userInfo 被篡改可导致白屏
- 文件：`web/src/store/user.js:9`

```js
userInfo: JSON.parse(localStorage.getItem('userInfo') || '{}')
```

非法 JSON 抛异常，Pinia 初始化失败。

修复：try-catch 封装安全 storage 读取。

### M-5 Nginx 代理超时与 Axios 超时不一致
- `request.js:9` timeout 30000（30s）；`web/nginx.conf:19-21` proxy_timeout 300s。LLM 慢接口 30s 即断，nginx 仍等后端。

修复：慢接口单独覆盖 axios timeout。

### M-6 无全局错误处理，Promise rejection 静默吞没
- `main.js` 无 `app.config.errorHandler`；
- `MainLayout.vue:140`、`purchase/orders.vue:272` 有 `.catch(()=>{})`。

修复：`app.config.errorHandler` + `window.addEventListener('unhandledrejection', ...)` 上报。

---

## 低

### L-1 ECharts 实例未在组件卸载时销毁
- 文件：`views/dashboard/index.vue:156-157,193-241`、`views/finance/cost.vue:90-91,124-161`、`views/finance/reports.vue:106-107,139-177`

无 `onBeforeUnmount` 调 `chart.dispose()`，无 `removeEventListener('resize')`，SPA 反复切换泄漏。

### L-2 登录页无暴力破解防护
无验证码、无尝试次数、无锁定（前端层面）。后端应有 rate limit，前端可加 CAPTCHA。

### L-3 入库/出库数量允许为 0
- `views/warehouse/inbound.vue:43`、`outbound.vue:43`、`production/outbound.vue:48`、`purchase/inbound.vue:49`

`:min="0"` 允许 0 数量，应改 `:min="1"`。

### L-4 出入库表单未重置仓库/库位
- `views/warehouse/inbound.vue:169-174` `resetForm` 不重置 warehouse_id/location_id，下次可能选错。

### L-5 Nginx 未配置静态资源缓存头
无 expires/Cache-Control，每次重下 JS/CSS。修复：`/assets/*` 设 `expires 1y`，`index.html` 设 `no-cache`。

---

## 提示（做得好的地方）

- **无 v-html / innerHTML**：全项目搜索为空。LLM 返回用 `{{ msg.content }}` 文本插值，Vue 自动转义，XSS 直接风险低。
- **Source Map 已关闭**：`vite.config.js:37 sourcemap: false`，生产不暴露源码。
- **无硬编码 API 密钥/AK-SK**。
- **Nginx SPA 回退正确**：`try_files $uri $uri/ /index.html`，刷新子路由不 404。
- **Gzip 已启用**。
- **依赖版本较新**：Vue 3.5.40、Element Plus 2.14.3、Vite 5.4.21、Axios 1.18.1，未发现已知高危 CVE。
- **审批/下达走专用端点**：`POST /purchase/orders/{orderNo}/approve`、`POST /production/work-orders/{woNo}/release`，而非前端直接 PUT status。
- **金额不在前端计算**：`total_amount` 由后端生成，避免浮点精度问题。

---

## 总结

前端处于"原型/Demo 完成度"。最突出的三个问题：

1. **权限体系完全空白**——路由、菜单、按钮三级均无控制，`getPermissions` 接口定义了但从未调用。
2. **默认管理员凭据硬编码并展示**——`admin/admin123` 在源码和界面上明文可见。
3. **错误处理逻辑颠倒**——几乎所有 API 调用的 catch 都提示"成功"并展示硬编码假数据，ERP 场景可能导致严重误操作。

XSS 直接风险较低（无 v-html、文本插值），但 token 存 localStorage 且无 CSP，纵深防御不足。表单校验大面积缺失。

### Top 5 优先修复
| 优先级 | 编号 | 问题 | 工作量 |
|---|---|---|---|
| 1 | S-1 | 删除登录页硬编码 admin/admin123 | 极小 |
| 2 | S-2 | 实现路由+菜单+按钮三级权限 | 大 |
| 3 | H-3 | 删除 catch 中"成功"提示和 mock 数据 | 中 |
| 4 | H-1 | Axios 拦截器处理 HTTP 401 | 小 |
| 5 | H-2 | 为所有表单添加校验规则 | 中 |
