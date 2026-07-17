# Copyright 2026 zhouhouping. All Rights Reserved.

FROM docker.m.daocloud.io/library/golang:1.21-alpine AS builder

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk --no-cache add git

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum* ./
RUN go mod tidy

COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o erp-server ./cmd/server

FROM docker.m.daocloud.io/library/alpine:latest

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /app/erp-server .
COPY configs ./configs
COPY migrations ./migrations

EXPOSE 8080

CMD ["./erp-server"]