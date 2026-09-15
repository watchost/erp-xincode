.PHONY: build build-migrate run test test-it migrate-up migrate-down migrate-version lint vet vuln clean

# 数据库连接参数（可被环境变量覆盖），供迁移与集成测试使用
DB_HOST ?= localhost
DB_PORT ?= 5432
DB_USER ?= erp_user
DB_PASSWORD ?= erp_password_zhouhouping_2026
DB_NAME ?= erp_system
DB_SSLMODE ?= disable
MIGRATE_DSN := host=$(DB_HOST) port=$(DB_PORT) user=$(DB_USER) password=$(DB_PASSWORD) dbname=$(DB_NAME) sslmode=$(DB_SSLMODE)

build:
	CGO_ENABLED=0 go build -o bin/server ./cmd/server

build-migrate:
	CGO_ENABLED=0 go build -o bin/migrate ./cmd/migrate

run:
	go run ./cmd/server

# 纯单元测试；集成测试由 test-it 触发（需要可用的 Postgres，见 internal/testsupport）
test:
	go test ./...

test-it:
# 不设 TEST_DATABASE_DSN 时，testsupport 会自动为每个包创建独立测试库（需 CREATEDB 权限）
	DB_HOST=$(DB_HOST) DB_PORT=$(DB_PORT) DB_USER=$(DB_USER) DB_PASSWORD='$(DB_PASSWORD)' go test -count=1 ./...

vet:
	go vet ./...

migrate-up: build-migrate
	DB_HOST=$(DB_HOST) DB_PORT=$(DB_PORT) DB_USER=$(DB_USER) DB_PASSWORD='$(DB_PASSWORD)' DB_NAME=$(DB_NAME) DB_SSLMODE=$(DB_SSLMODE) ./bin/migrate up

migrate-down: build-migrate
	DB_HOST=$(DB_HOST) DB_PORT=$(DB_PORT) DB_USER=$(DB_USER) DB_PASSWORD='$(DB_PASSWORD)' DB_NAME=$(DB_NAME) DB_SSLMODE=$(DB_SSLMODE) ./bin/migrate down

migrate-version: build-migrate
	DB_HOST=$(DB_HOST) DB_PORT=$(DB_PORT) DB_USER=$(DB_USER) DB_PASSWORD='$(DB_PASSWORD)' DB_NAME=$(DB_NAME) DB_SSLMODE=$(DB_SSLMODE) ./bin/migrate version

lint:
	golangci-lint run ./...

vuln:
	govulncheck ./...

clean:
	rm -rf bin/
