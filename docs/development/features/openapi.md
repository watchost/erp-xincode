# OpenAPI 第三方对接

## 1. 目标
提供标准 OAuth2 Client Credentials 鉴权，第三方可同步主数据、推送单据、订阅 Webhook。

## 2. 数据模型（现有表增强）

`open_client`：
- `client_secret_hash`（bcrypt 哈希，不存明文）；
- `allowed_scopes`（JSONB，允许的 scope 列表）；
- `allowed_ips`（JSONB，IP 白名单）；
- `rate_limit`（每分钟请求数）。

`open_token`：
- access token 用 UUID，SHA-256 哈希后存储；
- 过期时间；
- scope。

`open_webhook`：
- URL（校验 scheme + 禁止内网）；
- event 订阅；
- secret（HMAC 签名）。

## 3. 接口

### 3.1 OAuth2
```
POST /openapi/v1/oauth/token
  grant_type=client_credentials
  client_id=xxx
  client_secret=xxx
  scope=read write
→ { access_token, refresh_token, expires_in, scope }
```

### 3.2 资源接口（Bearer token 鉴权）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /openapi/v1/materials | 物料列表（增量同步） |
| GET | /openapi/v1/materials/:code | 物料详情 |
| GET | /openapi/v1/inventory | 库存查询 |
| GET | /openapi/v1/warehouses | 仓库 |
| POST | /openapi/v1/purchase-orders | 创建采购订单 |
| GET | /openapi/v1/purchase-orders/:no | 订单状态 |
| POST | /openapi/v1/sales-orders | 创建销售订单 |
| GET | /openapi/v1/sales-orders/:no | |

### 3.3 Webhook
- 业务事件（订单审批、入库完成、库存预警）POST 到订阅 URL；
- Header: `X-Webhook-Signature: sha256=HMAC(body, secret)`；
- 异步 worker 投递，失败重试 3 次（指数退避），最终进死信表；
- URL 校验：必须 http/https，禁止 127.0.0.1/169.254/10/172.16/192.168 等内网地址。

## 4. 关键逻辑

### 4.1 鉴权中间件
```go
func OpenAPIAuth(tokenRepo TokenRepo) gin.HandlerFunc {
    return func(c *gin.Context) {
        token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
        tok := tokenRepo.FindByAccessToken(sha256(token))
        if tok == nil || tok.ExpiresAt.Before(time.Now()) {
            abort(401)
            return
        }
        if !scopeAllowed(tok.Scope, requiredScope) {
            abort(403)
            return
        }
        c.Set("client_id", tok.ClientID)
        c.Next()
    }
}
```

### 4.2 client_id 来源
从 token 记录中取，**不接受 body 传入**（修复当前安全漏洞）。

### 4.3 限流
每 client_id 用 Redis 滑动窗口限流。

## 5. 测试要点
- 无效/过期 token 返回 401；
- scope 不足返回 403；
- webhook 签名验证；
- webhook 投递失败重试；
- SSRF 防护（内网 URL 被拒）。
