# 裸机部署（systemd）

适用于无 Docker 的环境（如当前开发机）。Docker Compose 方式见仓库根目录 `docker-compose.yml`。

## 安装（一次性）

```bash
# 1. 构建产物
CGO_ENABLED=0 go build -o bin/server  ./cmd/server
CGO_ENABLED=0 go build -o bin/migrate ./cmd/migrate

# 2. 应用数据库迁移
DB_HOST=localhost DB_USER=erp_user DB_NAME=erp_system ./bin/migrate up

# 3. 安装 systemd unit 并开机自启
sudo cp deploy/erp-server.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now erp-server
```

## 密钥

- 开发环境：不提供 `.env` 时使用 `configs/config.yaml` 与代码内开发默认值。
- 生产环境：创建 `.env`（已在 `.gitignore`），至少设置强随机 `JWT_SECRET`（≥32 字节）与真实 `DB_PASSWORD`：

  ```bash
  openssl rand -base64 48   # 生成 JWT_SECRET
  ```

  支持的环境变量：`SERVER_PORT`、`DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME/DB_SSLMODE`、
  `REDIS_HOST/REDIS_PORT/REDIS_PASSWORD/REDIS_DB`、`JWT_SECRET`、
  `JWT_ACCESS_EXPIRE`、`JWT_REFRESH_EXPIRE`。

## 常用运维命令

```bash
systemctl status erp-server      # 状态
journalctl -u erp-server -f      # 实时日志
systemctl restart erp-server     # 重启（发新版：重新 go build 后 restart）
systemctl stop erp-server        # 停服
curl http://localhost:8080/healthz
```

更新版本的标准流程：`go build` → `./bin/migrate up` → `systemctl restart erp-server`。
