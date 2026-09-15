// Copyright 2026 zhouhouping. All Rights Reserved.

// Package migrate 基于 golang-migrate 的数据库版本管理。
// 迁移 SQL 已通过 embed 内嵌进二进制，部署时只需执行 `erp-migrate up`
// （或配置 MIGRATE_AUTO=true 让服务启动时自动迁移）。
package migrate

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql 注册 "pgx" 驱动

	"erp-system/migrations"
)

// New 用内嵌的迁移脚本构造迁移器。dbURL 支持 pgx 的关键字 DSN
// （host=... user=... dbname=...）或 postgres:// URL。
func New(dbURL string) (*migrate.Migrate, error) {
	src, err := iofs.New(migrations.SQLFiles, ".")
	if err != nil {
		return nil, fmt.Errorf("加载内嵌迁移脚本失败: %w", err)
	}

	sqlDB, err := sql.Open("pgx", dbURL)
	if err != nil {
		return nil, fmt.Errorf("打开迁移数据库连接失败: %w", err)
	}

	driver, err := pgxmigrate.WithInstance(sqlDB, &pgxmigrate.Config{})
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("初始化迁移驱动失败: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		_ = driver.Close()
		return nil, fmt.Errorf("初始化迁移器失败: %w", err)
	}
	return m, nil
}

// Up 应用所有未执行的迁移；数据库已是最新时返回 nil（忽略 ErrNoChange）。
func Up(dbURL string) error {
	m, err := New(dbURL)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("执行迁移 up 失败: %w", err)
	}
	return nil
}

// Down 回滚全部迁移。仅用于本地/测试环境重建。
func Down(dbURL string) error {
	m, err := New(dbURL)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("执行迁移 down 失败: %w", err)
	}
	return nil
}

// Version 返回当前迁移版本与是否存在脏状态。
func Version(dbURL string) (version uint, dirty bool, err error) {
	m, err := New(dbURL)
	if err != nil {
		return 0, false, err
	}
	defer func() {
		_, _ = m.Close()
	}()
	return m.Version()
}
