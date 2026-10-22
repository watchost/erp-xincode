# 05 · P1 阶段（§2.1–§2.8）交付审计

> 审计范围：P1 七个提交（b37fa93 IAM、3c3ec0e MDM、3d0ec6d 采购/生产、92f4b6e 财务、a0af803 设备、1ef0829 LLM、6add7b8+207fe29 OpenAPI），全部结论已在 HEAD（207fe29）复核；严重/高条目由主审计逐条抽验或子审计实测复现。
> 分级标准：严重 = 资金/库存/凭证账实失真、进程可用性即时受损、存在完整提权或伪造链；高 = 安全语义落空或数据正确性缺陷需特定条件触发；中 = 边界健壮性/纵深防御缺口；低 = 设计折中与易修小问题。

---

## 一、严重

### S1 生产入库单位成本由调用方任意填写，污染库存计价与财务凭证，生产成本科目永不轧平
- 文件：`internal/production/service/service.go:485-530`、`internal/production/dto/dto.go:76`
- 完工入库的 `unit_cost` 仅校验 `decgt=0`，任何持 `production:receipt` 权限的用户可填任意正数；该值同时作为成品库存移动加权入库成本与「借 库存商品(1405)/贷 生产成本(5001)」过账金额。
- 场景：工单领料按实际加权成本借记 5001 共 10 万元，入库填 0.01 → 库存商品按几分钱入账、5001 残留约 10 万借方余额无人结转；填 9999 则虚增库存资产与利润。分批入库可逐批填不同价，无审计线索。dto 注释自述"P1 成本归集后改为实际结转"，属已知权宜但敞口为资金级。
- 修复：成本不得来自请求体。短期按该工单累计领料过账金额（5001 借方合计）/计划量推算或取主档标准成本；完工时补 5001 余额差异结转分录。

### S2 设备 WS 网关 send channel 的 close/send 竞态 → panic，且 WS 端口无 Recovery，进程直接崩溃
- 文件：`internal/device/gateway/gateway.go:163,272-279,343`、`cmd/server/main.go:367`
- `Send` 在 RUnlock 后操作 `c.send`，`close(c.send)` 与发送未串行化；慢连接缓冲满被 close 后仍留在 `h.conns`，下一次 `SendCommand` 即 send-on-closed panic；重连踢旧与在途发送并发则 double-close。WS 服务是裸 `http.HandlerFunc(deviceHub.HandleWS)`，无 gin Recovery——panic 发生在 WS goroutine 即进程退出。
- 修复：每连接 `done chan struct{}` + `sync.Once` 关闭，发送方 `select { case c.send<-data: case <-c.done: }`；踢旧/慢连接统一走 unregister+once.Close；WS handler 外包 recovery。

### S3 设备 ack 无归属校验：设备 A 可 ack 设备 B 的指令并唤醒 B 的 SendAndWait
- 文件：`internal/device/gateway/gateway.go:225-237,312-328`
- `type=ack` 仅按 AckID 查全局 pending 表即置消息成功，不校验消息的 DeviceID 与当前连接设备。message_id 为 `DEVMSG-<base36纳秒>-<全局递增序号>`，可推测同时段他设备在途 ID。
- 场景：恶意/被控设备 A 发送 `{type:"ack", ack_id:"<B的消息ID>"}` → B 的指令被落库成功、B 的 HTTP 调用方收到"已确认"，而 B 从未执行（对打印/开关类指令是实质安全事故）。
- 修复：ack 前 FindByMessageID 比对 `msg.DeviceID == c.device.ID`，不符丢弃并记审计；pending 表按 deviceCode 分桶双重校验。

### S4 期末结转汇总读数与防重检查在事务外，并发窗口漏结永久错账/双结裸 23505
- 文件：`internal/finance/service/accounting.go:230-250`
- ExistsPeriodClose 预检与三次 SumByAccount 汇总均在 WithTx 之外，事务只包 PostTx 写入。
- 场景 A（漏结）：汇总读数后、结转凭证提交前，一张当月采购入库恰好过账（PostTx 期间缺省=当月）→ 该笔分录永不被结转，而期间已被唯一索引标记"已结转"，4103 本年利润永久少计，无法补救。场景 B（双结）：两个并发 ClosePeriod 同期间都过预检，第二个撞 uk_fin_voucher_period_close，用户收到原始 23505（500）而非 60010。
- 修复：预检+汇总+写入同一事务，期间加 `pg_advisory_xact_lock` 或 SERIALIZABLE；唯一冲突翻译 60010；PostTx 增"已结转期间拒绝过账"守卫。

### S5 MDM UpdateCustomer 用 Save 全列覆盖，部分更新静默清风控字段（已实证）
- 文件：`internal/mdm/service/category_customer.go:176-187`、`internal/mdm/repository/repo.go:359-361`、`internal/mdm/handler/category_customer.go:104-117`
- handler 把请求体直接绑为 `model.MdmCustomer`，service 校验存在后 `Save` 全列。
- 场景（集成测试实测）：客户 credit_limit=8888/level=2/status=1，前端只发 `{"customer_code","name"}` → 更新后 credit_limit=0、level=0、status=0、attributes=NULL、created_at 被写成零值。风控额度静默清零。同包 UpdateCategory 是"先查再改"的正确写法，唯独客户没有。
- 修复：先 FindByID 取出现行，仅覆盖请求中出现的字段；或 `Updates`+map 指定列。

### S6 内置 admin（id=1）保护仅覆盖删除，iam:user 持有者可重置 admin 密码完成超管接管
- 文件：`internal/iam/service/user.go:225,245,149,203`
- 仅 DeleteUser 拦 id=1；ResetPassword/UpdateUser(停用)/AssignUserRoles 无守卫。
- 场景：HR 角色持 iam:user（无 iam:role）→ `POST /iam/users/1/reset-password`（空密码返回 12 位随机明文）→ 登录 admin → 强制改密白名单放行 change-password → 改为己有密码，接管超管。亦可禁用 admin 或剥其角色（DoS）。
- 修复：ResetPassword/UpdateUser/AssignUserRoles 对 id=1 加同等守卫；通用做法：禁止非 admin 角色操作 admin 账号。

---

## 二、高

### H1 改密/重置/登出后 refresh token 不失效，会话无法真正吊销
- `internal/iam/service/auth_flow.go:191,296`、`internal/iam/service/user.go:245`、`internal/iam/handler/handler.go:55`
- 改密/重置只写 `pwd:chg` 使旧 access 失效，`refresh:<uuid>` 不删、RefreshToken 也不比对 pwd:chg；Logout 仅拉黑 access jti。窃取 refresh token 后在受害者改密/被重置/登出后仍可无限换发新 access token，最长 refreshExpire（7d）。"改密踢下线/重置止损/登出"三语义全部落空。
- 修复：refresh 按用户建索引（`refresh:uid:<uid>` 集合），改密/重置/登出删除该用户全部 refresh key；或 RefreshToken 比对 key 写入时间与 pwd:chg。

### H2 强制改密标记 TTL=accessExpire，refresh 一次即永久绕过 ForcePasswordChange
- `internal/iam/service/auth_flow.go:96,253`、`internal/iam/service/service.go:158`
- `mustchg:<uid>` 仅 Login 时按 access TTL 同步，RefreshToken/issueTokens 不重建，中间件只查 Redis 不回源 DB。临时密码登录 → access 到期 → /refresh（公开端点）→ 新 access + mustchg 已过期 → 用临时密码无限期操作。
- 修复：mustchg 不带 TTL（或与 refreshExpire 对齐）并在 issueTokens 重建；中间件 Redis 缺失时回源 DB 判定。

### H3 期末结转后，被结转期间的利润表/财务简表本期数全部清零
- `internal/finance/service/accounting.go:310-333`、`internal/finance/service/service.go:159-170`
- 报表按凭证 period 过滤且不排除 `biz_type='period_close'` 的结转凭证（归属被结转期间）。8 月收入 1000 费用 300，ClosePeriod("2026-08") 后 GetIncomeStatement("2026-08") 返回全 0。集成测试只断言结转前，恰好掩盖。
- 修复：期间数报表取数 JOIN 加 `v.biz_type <> 'period_close'`；补结转后利润表断言。

### H4 webhook 投递 http.Client 默认跟随重定向，SSRF 校验被 307/308 完全绕过
- `internal/openapi/worker/worker.go:44,90-104`
- ValidatePublicURL 只校验 hook.URL；Go 默认最多跟随 10 次重定向，307/308 会带完整 body 与签名 Header 重发且不重新校验目标。攻击者将 webhook 指向自有公网服务器，307 到 `http://169.254.169.254/...` 或内网地址 → 业务 payload 原样 POST 进内网，M-6 修复形同虚设。
- 修复：`CheckRedirect` 直接拒绝；更彻底用自定义 DialContext 拨号时校验实际解析 IP（一并消除 H5 之外的 DNS 重绑定 TOCTOU，见 M16）。

### H5 OpenAPI access/refresh token 明文存库，违反 docs §2 与审计 L-6 约定
- `internal/openapi/service/service.go:125-144`、`internal/openapi/repository/repo.go:93-107`
- docs/development/features/openapi.md §2 明确"SHA-256 哈希后存储"，实现仍明文落库；DB 读泄露（注入/备份/只读账号）即获全部在效 token。对明文库做 `subtle.ConstantTimeCompare` 是无意义安慰。
- 修复：落库 sha256(token)、按哈希查询；存量随 2h TTL 自然过期或强制旋转。

### H6 物料更新在客户端不传 conversion_rate 时必然 500（0009 NOT NULL 列 × Save 全列更新的回归，已实证）
- `internal/mdm/service/service.go:135-137`、`internal/mdm/model/model.go:21`、migrations/0009
- `ConversionRate *decimal.Decimal` 为 nil 时 Save 写 NULL → 违反 NOT NULL（23502）→ 10300。任何按旧契约调 `PUT /mdm/materials/:id` 的客户端永久无法更新物料，且 Save 同 S5 毁 created_at。
- 修复：UpdateMaterial 先查再按字段合并，nil 保留原值；校验 conversion_rate > 0。

### H7 迁移 0010 辐射面漏改 dashboard，首页统计归零、状态描述全部"未知"
- `internal/dashboard/service/service.go:53-58,118-133`
- 状态码 1/2/5/3/4→10/20/30/40/50 后，Dashboard 仍按旧码：`status=1` 待审批计数、`status IN (2,3)` 进行中工单、purchaseStatusDesc case 1-5。迁移后三项统计恒 0/"未知"。
- 修复：改用 purchase/production 导出的状态常量，desc 同步新码。

### H8 迟到 ack 把终态"失败"翻回"成功"，消息状态机无终态保护
- `internal/device/gateway/gateway.go:226-236,312-328`
- ackMessage 无条件覆盖 status/AckedAt；pending 已删（ok=false）时仍执行 `ackMessage(id,true)`。SendAndWait 30s 超时置失败并返 504 后，第 31s 设备 ack 到达 → 流水显示成功、调用方却已按超时补发 → 设备重复执行。
- 修复：条件更新 `WHERE message_id=? AND status=0`，仅允许 待处理→成功/失败 单向流转。

### H9 WebSocket 连接从不 `ws.Close()`：fd 泄漏；踢旧不断旧 socket
- `internal/device/gateway/gateway.go:147-156,180-222`
- 全文件无一处 ws.Close()。设备断电 → 90s 读超时返回但 TCP fd 不回收，长跑 fd 耗尽；register 踢旧只 close(send)，旧 socket 与 readPump 存活最长 90s，期间旧连接上行与 ack 仍被处理（叠加 S3）。
- 修复：HandleWS `defer c.ws.Close()`；踢旧时 `old.ws.Close()`。

### H10 心跳清扫只改库不断连、pong 不刷库：hub 内存态与 DB 态双向分裂
- `internal/device/gateway/gateway.go:183-186,334-336`、`cmd/server/main.go:379-392`
- 设备只回 WS 层 ping/pong 不发应用心跳 → 60s 后 DB 置离线但 IsOnline=true 指令照发（列表显示离线的设备指令却能执行）；反向，被 sweep 置离线后任意上行用建连时旧结构体 Save 又"复活"为在线，sweep 形同虚设。
- 修复：SweepOffline 同时踢出注册表并 Close 超时连接；上行（节流）刷新 DB last_heartbeat，或以 hub 内存态为准由 sweep 统一对账，保持单一事实源。

### H11 OpenAPI refresh 旋转非原子，失败/并发下新旧两对 token 同时有效
- `internal/openapi/service/service.go:148-167`
- issueToken（插新行）与 DeleteByAccessToken（删旧行）两条独立语句，删除错误被 `_ =` 吞掉。Create 后崩溃 → 旧 refresh 可再换（旋转失效）；两并发 refresh 都在对方删除前通过查找 → 同一 refresh 产出两对有效 token。
- 修复：单事务 `DELETE ... WHERE refresh_token=? RETURNING` 后插新行，删除失败回滚；唯一约束兜底。

### H12 会话标题按字节截断 UTF-8，中文长提问建会话 500
- `internal/llm/service/service.go:305-310,57`
- `s[:50]` 按字节切割，50 字节边界大概率切断 3 字节中文字 → 非法 UTF-8 写入 PG text → `invalid byte sequence` → 首轮提问超约 17 个汉字即建会话失败、整次聊天 500。
- 修复：按 rune 截断或 `strings.ToValidUTF8` 兜底。

---

## 三、中

**IAM**
- M1 图形验证码答案空间仅 256：`captcha.go:34` `int(b[0])%10000` 单字节取模，爆破脚本每 captchaID 1/256 概率猜中，验证码形同虚设。修复：4 字节各取模 10。
- M2 未配置 TrustedProxies：`main.go:253` gin 默认信任 XFF，轮换 `X-Forwarded-For` 即绕过登录限流与 IP 锁定（密码喷洒无成本），审计留痕被污染。修复：`r.SetTrustedProxies(...)` 按部署形态配置。
- M3 迁移 0008 加可空 dept_id 未回填：`0008_iam_enhancement.up.sql:64-65` + 模型非指针 int64 → 存量单据 SELECT 即 Scan 报错 500。修复：迁移补 `UPDATE ... SET dept_id=0` + NOT NULL/DEFAULT。
- M4 data_scope claim 缺省 fail-open 为全量：`iam/service.go:236-238` 与 datascope 自身 fail-closed 原则相反，旧 token 窗口内全数据可见。修复：缺省按最严档。
- M5 权限画像全量冻结在 JWT：禁用/降权/删用户在 access 有效期内不生效。修复：压缩 accessExpire 或 CheckClaims 增查用户版本号。
- M6 CreateDepartment 非事务两段写物化路径：`iam/service/dept.go:36-40` path 残留 "/" 时 scope3（本部门及下级）`LIKE '/%'` 放大为全公司数据。修复：同事务。

**财务**
- M7 红字/负净额损益科目结转静默跳过：`accounting.go:254-277` 收入借余/费用贷余被 continue，期间却标记已结转，残留余额永无法结平。修复：负净额反方向结平或显式报错。
- M8 结转与利润表科目范围硬编码 60/64/66：`accounting.go:238-250,310-323` 6301/6701/6801 等损益科目既不进利润表也永不被结转。修复：按 fin_account.category 驱动。
- M9 PostTx 不校验分录科目存在/末级：`posting.go:143-150` 错码照样入账且绕过科目删除守卫（TOCTOU）。修复：强制校验存在且 is_leaf，守卫纳入事务。

**采购/生产**
- M10 BOM/工单物料行无去重：`production/service.go:157-168,194-210,378-384` 同物料多行时齐套逐行放行（需求不累计）、扫码领料只命中首行，重复行永远领不齐。修复：CreateBom 校验 MaterialID 唯一或建单时聚合；齐套按物料聚合需求。
- M11 齐套检查 check-then-act 无预留且口径为跨仓合计：两单同时通过超量下达（出库有守卫不致错账，仅业务受阻）。修复：文档明示软检查或引入预留量。
- M12 生产入库微小数量归整为 0 触发财务 500 且消耗幂等键：`purchase/service.go:365`、`production/service.go:394,484` qty=0.0004 → 归整 0 → PostTx 拒绝 + 同幂等键重试 429。修复：归整后判正，为 0 返回 400。

**MDM**
- M13 UpdateCustomer 改编码无查重，撞唯一索引返回裸 23505（10300）。修复：变更编码先 FindByCode。
- M14 客户校验 DTO（CustomerReq：decgte/level/status/长度）定义了却未接线，负信用额度等脏数据可入库。修复：handler 改绑 DTO。
- M15 单位换算字段全链路无人消费且无 `conversion_rate > 0` 校验：0/负数入库，未来换算逻辑除零。修复：至少应用层+DB CHECK 校验，或文档声明本期仅建字段。
- M16 DeleteCategory 引用守卫 TOCTOU：检查后删除前并发建子分类/物料 → 孤儿子树永不出现在树接口但占用唯一码，数据卡死。修复：检查+删除同事务并锁父行。

**设备**
- M17 api_secret_hash 本身即 HMAC 密钥：`gateway.go:83-96` 摘要=凭据等价物，库泄即可签名接入，"只存摘要"无安全收益。修复：信封加密存 secret 或改非对称签名；至少文档明示敏感度等同明文。
- M18 接入无防重放 + 明文 ws + 凭据走 query：`gateway.go:84-96` 5min 窗口内签名 URL 可重用且踢掉真设备（DoS）；query 进访问日志。修复：一次性消费缓存、生产强制 wss。
- M19 handleUp 用建连时旧结构体整体 Save：`gateway.go:239-241` 覆盖管理员的 status/固件等修改。修复：定向 Updates(last_heartbeat,last_ip)。

**LLM**
- M20 ChatStream 中途出错丢弃已生成部分答案换 fallback：`llm/service.go:103-109` 客户端先看到半段回答再收到整段"AI服务暂时不可用"，历史也只有 fallback。修复：err 时保留非空部分答案落库，连接已死不再 emit。
- M21 dashboard 场景 Prompt 模板是死代码：`dashboard/service.go:176-180` 种子模板从未被读，启停不生效。修复：DashboardService 注入 promptRepo 与 chat 路径对齐。
- M22 对话输入与历史无长度/token 上限：`llm/dto.go:7`、`llm/service.go:24` 1MB 问题原样进上下文+落库，成本失控；超大 SSE 帧触发 scanner ErrTooLong 又落 M20。修复：Question 加 max、binding 历史按预算裁剪。

**OpenAPI**
- M23 /openapi/v1 与 oauth/token 均无限流，违反 docs §4.3：`routes.go:108-122` client_secret 在线爆破无节流（bcrypt 还是 CPU DoS 放大器）。修复：token 端点 per-IP、v1 per-client_id 限流。
- M24 分页参数无上限，负数 page_size 使 GORM 取消 LIMIT 退化为全表返回：`openapi/handler/handler.go:117-142`。修复：clamp 1..200。
- M25 CreateOrder 绕过采购领域服务：`openapi/service/resource.go:131-195` 缺 CreatedBy/DeptID（数据权限不可见、无追溯）、无标度归整、无幂等、无审计。修复：复用 purchaseService 领域方法，补幂等键与审计。
- M26 禁用客户端不使其已签发 token 失效，`revoked` 列无任何置位入口：`openapi/service/service.go:169-184` 凭据泄露后最长 2h 无法止损。修复：校验客户端状态或提供 revoke API。
- M27 webhook 无生产事件源 + 订阅模型跨客户端泄露：`openapi/service/service.go:287-307` DispatchEvent 无业务调用方；ListByEvent 全局匹配，接入后任何订阅方可收全公司业务事件。修复：接入事件源并按客户端授权过滤。
- M28 SSRF DNS 重绑定 TOCTOU：`openapi/service/ssrf.go:64-73` 校验解析与拨号解析两次独立，低 TTL 域名可换 IP。修复：DialContext 拨号期校验实际 IP（与 H4 合并治理）。

---

## 四、低（一句话清单）

- IAM：admin 角色权限可被 iam:role 持有者清空（role.go:133 无守卫）；验证码 GET+DEL 非原子可并发双花（auth_flow.go:111）；DeleteDepartment 守卫 TOCTOU（dept.go:59）；禁用账号差异化文案可枚举账号（auth_flow.go:78）；用户名锁定可被定向 DoS（设计折中，建议监控）。
- 采购/生产：date_from/to 无格式校验非法值 500、日边界按 DB 会话时区（purchase/repo.go:87）；关闭工单复用创建权限点（routes.go:223，宜拆 production:wo:close）。
- 财务：预算跨月释放错位——月末占用、次月取消释放打到新月（service.go:246，前置提交 493e9f3 引入）。
- MDM：CreateCategory 建分类与路径回填非事务（category_customer.go:87）；物料不校验 category_id 存在性，可建孤儿物料（service.go:127）。
- 设备：读超时只由 pong 刷新，纯数据消息设备 90s 被误断（gateway.go:180）；SendAndWait 把"推送瞬间掉线"谎报为 504 超时（service.go:161）；tenant_id 恒 0，多租户仅装饰。
- LLM：api_key 未配置时 chat 不短路仍打外部 HTTPS（main.go:206）；Dashboard 分析把供应商/价格明文外发阿里云，无脱敏无审计（dashboard/service.go:194）；流式无总超时，上游半开时 goroutine 泄漏（gateway.go:132）；CreatePromptTemplate 忽略 req.Enabled 创建即全局生效（llm/service.go:238）；会话历史无分页（llm/service.go:184）。
- OpenAPI：markDead 对瞬时 DB/DNS 故障零重试直接死信（worker.go:79）；refresh token 无独立有效期（service.go:148）；GetOrder 明细 N+1（resource.go:220）；grant_type 不校验取值（dto.go:8）。

---

## 五、总体评价

P1 把系统从"接口能通"推进到"主链路可用"：采购/生产/财务的行锁串行化与同事务过账、IAM 五档数据权限的 fail-closed 设计、OpenAPI 的异步投递骨架，方向都正确，测试覆盖也较认真（MDM 子审计用真实 PG 实证、采购/财务的并发疑点经复核不成立者居多）。

但七个模块共通的短板是**异常路径与对抗场景**：状态翻转（迟到 ack、结转后报表）、事务边界（ClosePeriod、refresh 旋转、CreateDepartment）、"保存整行"式更新（S5/H6/M19 同源）、以及安全承诺只兑现一半（token 哈希、限流、模板启停、强制改密）。资金级敞口两条——S1（成本任意填）与 S4（结转漏结）——建议在 P2 开工前修复；S2/S3 的设备链路崩溃与伪造、S6 的提权链同属必修。

**建议修复顺序（P1.5 收口迭代）**：
1. 资金与进程安全：S1、S4、S2、S3（均可独立小步修复，各配回归测试）；
2. 提权与吊销链：S6、H1、H2（IAM 会话生命周期一次收口）；
3. 数据正确性回归：S5、H6、H7、H3（各 1-2 天）；
4. 安全承诺补齐：H4+M28（拨号期校验统一方案）、H5、H11、M23；
5. 中低频条目随 P2 相关模块顺带治理。

**各模块"经核实不成立"的疑点**（避免重复排查）：采购/生产的行锁并发、同事务回滚、日期注入面；财务的借贷平衡强制、凭证号唯一、迁移回填；IAM 的审计打码、datascope 五档过滤、软删重建；设备的幂等落库与 owner 校验踢旧；LLM 的 IDOR 全覆盖与 SSE 资源回收；OpenAPI 的十进制 IP 绕过、IPv4-mapped IPv6、投递 at-least-once 语义——均已在本次审计中排除。
