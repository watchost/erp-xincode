// Copyright 2026 zhouhouping. All Rights Reserved.

// Package migrations 内嵌全部数据库迁移 SQL，随服务二进制一起发布，
// 不再依赖 docker-entrypoint-initdb.d（后者只在数据卷首次初始化时执行，
// 后续迁移永远不会生效）。
package migrations

import "embed"

// SQLFiles 保存 0001 起的成对 up/down 迁移脚本（golang-migrate 命名规范）。
//
//go:embed *.up.sql *.down.sql
var SQLFiles embed.FS
