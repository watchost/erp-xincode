// Copyright 2026 zhouhouping. All Rights Reserved.

// Package tenant 实现 P3 §4.2 多租户隔离的 GORM 插件：
//   - 查询/更新/删除/Row：对含 tenant_id 列的表自动追加 `<表或别名>.tenant_id = ?`；
//   - 创建：强制把 TenantID 字段写成上下文租户（忽略调用方传入的值，防止伪造）；
//   - 生效前提：context 中显式设置了租户（auth.WithTenantID），且未 WithSkipTenant。
//
// 已知限制（如实记录）：插件依赖 statement 的 context。当前大部分 repository 方法
// 使用不带 ctx 的 r.db 直接查询，这些查询**不会**被过滤；事务内（TxManager.WithTx 传入 ctx）
// 与 db.WithContext(ctx) 的查询会被过滤。完整隔离需要 repository 层透传 ctx，见 docs/development/api.md §13.5。
package tenant

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"erp-system/internal/pkg/auth"
)

const pluginPrefix = "tenant:"

// Plugin 保存含 tenant_id 列的表集合（启动时从 information_schema 读取）。
type Plugin struct {
	mu     sync.RWMutex
	tables map[string]bool
}

func (p *Plugin) Name() string { return "erp-tenant" }

// Initialize 读取表集合并注册回调；对同一 *gorm.DB 重复注册是幂等的（测试会多次调用）。
func (p *Plugin) Initialize(db *gorm.DB) error {
	if err := p.Reload(db); err != nil {
		return err
	}
	q := db.Callback().Query()
	if q.Get(pluginPrefix+"query") != nil {
		return nil
	}
	if err := q.Before("gorm:query").Register(pluginPrefix+"query", p.filter); err != nil {
		return err
	}
	if err := db.Callback().Row().Before("gorm:row").Register(pluginPrefix+"row", p.filter); err != nil {
		return err
	}
	if err := db.Callback().Update().Before("gorm:update").Register(pluginPrefix+"update", p.filterWrite); err != nil {
		return err
	}
	if err := db.Callback().Delete().Before("gorm:delete").Register(pluginPrefix+"delete", p.filterWrite); err != nil {
		return err
	}
	return db.Callback().Create().Before("gorm:create").Register(pluginPrefix+"create", p.stamp)
}

// Reload 重新读取含 tenant_id 列的表（迁移之后调用）。
func (p *Plugin) Reload(db *gorm.DB) error {
	var names []string
	if err := db.Raw(`SELECT table_name FROM information_schema.columns
		WHERE table_schema = current_schema() AND column_name = 'tenant_id'`).Scan(&names).Error; err != nil {
		return err
	}
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	p.mu.Lock()
	p.tables = m
	p.mu.Unlock()
	return nil
}

func (p *Plugin) tracked(table string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tables[table]
}

// Register 安装插件；同一个 *gorm.DB 已安装时直接返回。
func Register(db *gorm.DB) (*Plugin, error) {
	if existing, ok := db.Config.Plugins["erp-tenant"]; ok {
		pl := existing.(*Plugin)
		return pl, pl.Reload(db)
	}
	pl := &Plugin{}
	return pl, db.Use(pl)
}

// tenantOf 返回需要应用的租户；第二个返回值 false 表示不过滤。
func tenantOf(db *gorm.DB) (int64, bool) {
	ctx := db.Statement.Context
	if auth.SkipTenant(ctx) {
		return 0, false
	}
	return auth.TenantFromContext(ctx)
}

// RealTable 解析 statement 实际操作的表名与用于限定列的引用名（别名优先）。
func RealTable(stmt *gorm.Statement) (real, ref string) {
	if stmt.TableExpr != nil && stmt.TableExpr.SQL != "" {
		fields := strings.Fields(strings.ReplaceAll(stmt.TableExpr.SQL, `"`, ""))
		if len(fields) > 0 {
			real = fields[0]
			ref = fields[len(fields)-1]
			if len(fields) >= 3 && strings.EqualFold(fields[1], "AS") {
				ref = fields[2]
			}
			return real, ref
		}
	}
	if stmt.Table != "" {
		return stmt.Table, stmt.Table
	}
	if stmt.Schema != nil {
		return stmt.Schema.Table, stmt.Schema.Table
	}
	return "", ""
}

func (p *Plugin) apply(db *gorm.DB) {
	tid, ok := tenantOf(db)
	if !ok {
		return
	}
	stmt := db.Statement
	if stmt.SQL.Len() > 0 { // db.Raw/Exec 已拼好 SQL，无法注入，见包注释
		return
	}
	real, ref := RealTable(stmt)
	if real == "" || !p.tracked(real) {
		return
	}
	stmt.AddClause(clause.Where{Exprs: []clause.Expression{
		clause.Eq{Column: clause.Column{Table: ref, Name: "tenant_id"}, Value: tid},
	}})
}

func (p *Plugin) filter(db *gorm.DB) { p.apply(db) }

// filterWrite 仅在已有 WHERE 时追加租户条件；没有 WHERE 的全表更新/删除仍交给 GORM 自身拦截。
func (p *Plugin) filterWrite(db *gorm.DB) {
	if _, has := db.Statement.Clauses["WHERE"]; !has {
		return
	}
	p.apply(db)
}

// stamp 创建时强制写入上下文租户。表含 tenant_id 但模型没有 TenantID 字段且租户非 0 时报错（fail-closed），
// 否则该行会以默认值 0 落库，变成其他租户可见的数据。
func (p *Plugin) stamp(db *gorm.DB) {
	tid, ok := tenantOf(db)
	if !ok || db.Statement.Schema == nil {
		return
	}
	stmt := db.Statement
	real, _ := RealTable(stmt)
	if !p.tracked(real) {
		return
	}
	field := stmt.Schema.LookUpField("TenantID")
	if field == nil {
		if tid != 0 {
			_ = db.AddError(fmt.Errorf("tenant: 表 %s 含 tenant_id，但模型 %s 没有 TenantID 字段，拒绝写入以免跨租户泄漏", real, stmt.Schema.Name))
		}
		return
	}
	rv := stmt.ReflectValue
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			if err := field.Set(stmt.Context, rv.Index(i), tid); err != nil {
				_ = db.AddError(err)
				return
			}
		}
	case reflect.Struct:
		if err := field.Set(stmt.Context, rv, tid); err != nil {
			_ = db.AddError(err)
		}
	}
}
