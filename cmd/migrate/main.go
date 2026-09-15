// Copyright 2026 zhouhouping. All Rights Reserved.

// erp-migrate：数据库迁移命令。
//
//	erp-migrate up        应用全部未执行的迁移
//	erp-migrate down      回滚全部迁移（危险，仅本地/测试）
//	erp-migrate version   查看当前版本与脏状态
//	erp-migrate force N   强制清理脏版本标记并指向版本 N
//
// 连接参数与服务一致，从环境变量读取：
// DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME/DB_SSLMODE
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	migratetool "erp-system/internal/pkg/migrate"
)

func main() {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		env("DB_HOST", "localhost"),
		env("DB_PORT", "5432"),
		env("DB_USER", "erp_user"),
		env("DB_PASSWORD", "erp_password_zhouhouping_2026"),
		env("DB_NAME", "erp_system"),
		env("DB_SSLMODE", "disable"),
	)

	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "up":
		if err := migratetool.Up(dsn); err != nil {
			log.Fatalf("migrate up failed: %v", err)
		}
		log.Println("migrate up: database is up to date")

	case "down":
		if err := migratetool.Down(dsn); err != nil {
			log.Fatalf("migrate down failed: %v", err)
		}
		log.Println("migrate down: all migrations rolled back")

	case "version":
		version, dirty, err := migratetool.Version(dsn)
		if err != nil {
			log.Fatalf("migrate version failed: %v", err)
		}
		log.Printf("migration version=%d dirty=%v", version, dirty)

	case "force":
		if len(os.Args) < 3 {
			log.Fatal("usage: erp-migrate force <version>")
		}
		v, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatalf("invalid version %q: %v", os.Args[2], err)
		}
		m, err := migratetool.New(dsn)
		if err != nil {
			log.Fatalf("init migrator failed: %v", err)
		}
		defer func() { _, _ = m.Close() }()
		if err := m.Force(v); err != nil {
			log.Fatalf("migrate force failed: %v", err)
		}
		log.Printf("migration version forced to %d", v)

	default:
		log.Fatalf("unknown command %q (supported: up|down|version|force)", cmd)
	}
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
