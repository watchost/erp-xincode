# Copyright 2026 zhouhouping. All Rights Reserved.

FROM golang:1.27-alpine AS builder

RUN apk --no-cache add git

WORKDIR /app

# 国内构建默认走 goproxy.cn，可通过 --build-arg GOPROXY=... 覆盖
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# 迁移 SQL 通过 go:embed 内嵌进二进制（migrations/fs.go）
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o erp-server ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o erp-migrate ./cmd/migrate

FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -S app && adduser -S -G app app
ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /app/erp-server .
COPY --from=builder /app/erp-migrate .
COPY configs ./configs

# 以非 root 用户运行（审计 P0 安全加固）。
RUN chown -R app:app /app
USER app

EXPOSE 8080

CMD ["./erp-server"]
