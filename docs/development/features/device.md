# 设备管理与硬件对接

## 1. 目标
真实实现设备 WebSocket 接入、心跳、指令下发、扫码/RFID/PDA/打印机适配器对接。

## 2. 数据模型
见 [schema.md P1-2](../schema.md#p1-2-设备模块)。
- `dev_device` 增加 api_key/api_secret_hash/last_ip/firmware_version；
- `dev_device_message` 消息流水。

## 3. 架构

```
设备 ──WebSocket──▶ Device Gateway (8081)
                      │
                      ├── 鉴权（api_key + 签名 + 时间戳）
                      ├── 心跳维护在线状态
                      ├── 消息路由到 Adapter
                      └── 指令下发通道

业务系统 ──HTTP──▶ Device Handler ──▶ Device Service ──▶ WebSocket Hub ──▶ 设备
```

### 3.1 WebSocket Hub
- 维护 `map[deviceCode]*Connection`；
- 注册/注销/广播；
- 读 goroutine + 写 goroutine，每连接一个 pump；
- 心跳超时（60s 未收到心跳）关闭连接，标记离线。

### 3.2 鉴权
连接 URL：
```
wss://host:8081/ws?api_key=xxx&timestamp=xxx&sign=xxx
```
sign = HMAC-SHA256(api_secret, api_key + timestamp)，时间戳偏差 ≤5 分钟。

## 4. 接口

### 4.1 管理接口（HTTP，走标准 JWT）
| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | /devices | device:register | 注册设备（生成 api_key/secret） |
| GET | /devices | device:view | 设备列表（在线状态） |
| GET | /devices/:code | | 详情 |
| POST | /devices/:code/command | device:command | 下发指令 |
| GET | /devices/:code/messages | | 消息记录 |

### 4.2 设备接口（WebSocket）
设备上行消息：
```json
{ "type": "heartbeat", "ts": 1234567890, "data": {...} }
{ "type": "scan", "ts": ..., "data": { "code": "MAT001", "type": "barcode" } }
{ "type": "rfid_read", "data": {...} }
{ "type": "ack", "message_id": "...", "status": "ok" }
```

服务端下行：
```json
{ "type": "command", "message_id": "...", "action": "scan", "params": {} }
{ "type": "print", "params": {"label": "..."} }
```

## 5. Adapter

### 5.1 ScannerAdapter
- 接收扫码枪输入（USB HID 或网络扫码枪）；
- 解析条码，发送到 `device.scan` topic；
- 业务系统订阅后触发入库/出库扫码逻辑。

### 5.2 RFIDAdapter
- 批量读取 EPC 标签；
- 支持盘点、出入库批量扫描；
- 防碰撞算法在硬件层，软件只收结果。

### 5.3 PDAAdapter
- PDA 通过 WebSocket 连接；
- 发送扫码结果、接收任务指令；
- 支持离线缓存（PDA 本地暂存，联网重传，带幂等键）。

### 5.4 PrinterAdapter
- 接收标签打印指令；
- 生成条码/二维码图片（boombuler/barcode）；
- 发送到网络打印机（ESC/POS 或 ZPL）。

## 6. 关键逻辑

### 6.1 在线状态
- 设备连接 → 标记 online，更新 last_heartbeat；
- 每 10s 扫描，超过 60s 未心跳 → 标记 offline 并关闭连接；
- 状态变更写消息流水。

### 6.2 扫码与业务联动
设备 scan 消息到达后，根据设备绑定的仓库/库位和当前模式（入库/出库/盘点），调用对应 service 的扫码方法。设备端通过下行指令切换模式。

### 6.3 指令下发
- 写 `dev_device_message`（direction=下发，status=待处理）；
- 通过 WebSocket 推送到设备；
- 设备回 ack 后更新 status；
- 超时（30s）未 ack 标记失败，可重试。

## 7. 边界条件
- 同一设备重复连接：踢掉旧连接；
- 消息乱序：用 message_id 去重；
- 设备时钟不准：时间戳偏差 ≤5 分钟；
- WebSocket 断线：设备自动重连，消息漏发通过查询接口补传。

## 8. 测试要点
- 设备鉴权失败拒绝连接；
- 心跳超时正确标记离线；
- 指令下发 ack 机制；
- 扫码消息正确路由到业务 service；
- 100 设备并发连接无 goroutine 泄漏。
