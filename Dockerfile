# Copyright 2026 zhouhouping. All Rights Reserved.

FROM golang:1.21-alpine AS builder

RUN apk --no-cache add git

WORKDIR /app

ENV GOPROXY=https://proxy.golang.org,direct

COPY go.mod go.sum* ./
RUN go mod tidy

COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o erp-server ./cmd/server

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /app/erp-server .
COPY configs ./configs
COPY migrations ./migrations

EXPOSE 8080

CMD ["./erp-server"]