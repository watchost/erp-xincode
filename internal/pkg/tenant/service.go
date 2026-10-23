// Copyright 2026 zhouhouping. All Rights Reserved.

package tenant

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"erp-system/internal/pkg/auth"
	"erp-system/internal/pkg/errors"
)

// SysTenant 租户。id=0 为默认/平台租户。
type SysTenant struct {
	ID        int64           `json:"id"`
	Code      string          `json:"code"`
	Name      string          `json:"name"`
	Status    int16           `json:"status"`
	ExpireAt  *time.Time      `json:"expire_at"`
	Config    json.RawMessage `gorm:"type:jsonb" json:"config"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func (SysTenant) TableName() string { return "sys_tenant" }

const (
	StatusEnabled  = 1
	StatusDisabled = 0
)

// 错误码段 81xxx：租户
var (
	ErrTenantNotFound = errors.New(81001, 404, "租户不存在")
	ErrTenantDisabled = errors.New(81002, 403, "租户已停用")
	ErrTenantExpired  = errors.New(81003, 403, "租户已过期，禁止写操作")
	ErrPlatformOnly   = errors.New(81004, 403, "仅平台（默认租户）管理员可操作租户")
	ErrTenantCodeDup  = errors.New(81005, 409, "租户编码已存在")
	ErrUsernameDup    = errors.New(81006, 409, "管理员用户名已被使用（用户名全局唯一）")
)

type CreateReq struct {
	Code          string          `json:"code" binding:"required,max=64"`
	Name          string          `json:"name" binding:"required,max=128"`
	ExpireAt      *time.Time      `json:"expire_at"`
	Config        json.RawMessage `json:"config"`
	AdminUsername string          `json:"admin_username" binding:"required,min=3,max=64"`
	AdminPassword string          `json:"admin_password" binding:"required,min=8,max=72"`
}

type UpdateReq struct {
	Name     *string         `json:"name" binding:"omitempty,max=128"`
	ExpireAt *time.Time      `json:"expire_at"`
	Config   json.RawMessage `json:"config"`
}

type StatsVO struct {
	TenantID       int64 `json:"tenant_id"`
	Users          int64 `json:"users"`
	Materials      int64 `json:"materials"`
	PurchaseOrders int64 `json:"purchase_orders"`
	SalesOrders    int64 `json:"sales_orders"`
}

// Service 租户管理 + 租户状态检查（带 30 秒缓存）。
type Service struct {
	db    *gorm.DB
	mu    sync.Mutex
	cache map[int64]cached
}

type cached struct {
	t  SysTenant
	at time.Time
}

const cacheTTL = 30 * time.Second

func NewService(db *gorm.DB) *Service {
	return &Service{db: db, cache: map[int64]cached{}}
}

func requirePlatform(ctx context.Context) error {
	tid, ok := auth.TenantFromContext(ctx)
	if !ok || tid != 0 {
		return ErrPlatformOnly
	}
	return nil
}

// platformDB 平台接口显式绕过租户过滤（独立权限 + 仅默认租户可调用）。
func (s *Service) platformDB(ctx context.Context) *gorm.DB {
	return s.db.WithContext(auth.WithSkipTenant(ctx))
}

func (s *Service) List(ctx context.Context) ([]SysTenant, error) {
	if err := requirePlatform(ctx); err != nil {
		return nil, err
	}
	var list []SysTenant
	return list, s.platformDB(ctx).Order("id").Find(&list).Error
}

// copyTenantRows 把租户 0 的种子数据复制给新租户（列名从 information_schema 读取，排除 id/tenant_id）。
func copyTenantRows(tx *gorm.DB, table string, newTenant int64, where string) error {
	var cols []string
	if err := tx.Raw(`SELECT column_name FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = ? AND column_name NOT IN ('id','tenant_id')
		ORDER BY ordinal_position`, table).Scan(&cols).Error; err != nil {
		return err
	}
	if len(cols) == 0 {
		return fmt.Errorf("表 %s 不存在或无可复制列", table)
	}
	list := `"` + strings.Join(cols, `","`) + `"`
	sql := fmt.Sprintf(`INSERT INTO %s (tenant_id, %s) SELECT ?, %s FROM %s WHERE tenant_id = 0`, table, list, list, table)
	if where != "" {
		sql += " AND " + where
	}
	return tx.Exec(sql, newTenant).Error
}

// Create 新建租户：复制科目表（含 parent_id 重映射）与已部署的流程定义，并创建该租户的 admin 用户（绑定 admin 角色）。
func (s *Service) Create(ctx context.Context, req CreateReq) (*SysTenant, error) {
	if err := requirePlatform(ctx); err != nil {
		return nil, err
	}
	hash, err := auth.HashPassword(req.AdminPassword)
	if err != nil {
		return nil, err
	}
	cfg := req.Config
	if len(cfg) == 0 {
		cfg = json.RawMessage(`{}`)
	}
	t := &SysTenant{Code: req.Code, Name: req.Name, Status: StatusEnabled, ExpireAt: req.ExpireAt,
		Config: cfg, CreatedAt: time.Now(), UpdatedAt: time.Now()}

	err = s.platformDB(ctx).Transaction(func(tx *gorm.DB) error {
		var dup int64
		tx.Model(&SysTenant{}).Where("code = ?", req.Code).Count(&dup)
		if dup > 0 {
			return ErrTenantCodeDup
		}
		tx.Table("sys_user").Where("username = ? AND deleted_at IS NULL", req.AdminUsername).Count(&dup)
		if dup > 0 {
			return ErrUsernameDup
		}
		if err := tx.Create(t).Error; err != nil {
			return err
		}
		if err := copyTenantRows(tx, "fin_account", t.ID, ""); err != nil {
			return err
		}
		// 复制后的 parent_id 仍指向租户 0 的科目，按科目编码重新映射到本租户的科目
		if err := tx.Exec(`UPDATE fin_account n SET parent_id = np.id
			FROM fin_account op, fin_account np
			WHERE n.tenant_id = ? AND n.parent_id = op.id AND op.tenant_id = 0
			  AND np.tenant_id = ? AND np.code = op.code`, t.ID, t.ID).Error; err != nil {
			return err
		}
		if err := copyTenantRows(tx, "wf_definition", t.ID, "status = 1"); err != nil {
			return err
		}
		var uid int64
		if err := tx.Raw(`INSERT INTO sys_user (username, password_hash, real_name, phone, status, tenant_id)
			VALUES (?, ?, ?, '', 1, ?) RETURNING id`, req.AdminUsername, hash, req.Name+"管理员", t.ID).Scan(&uid).Error; err != nil {
			return err
		}
		return tx.Exec(`INSERT INTO sys_user_role (user_id, role_id) SELECT ?, id FROM sys_role WHERE code = 'admin'`, uid).Error
	})
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) find(db *gorm.DB, id int64) (*SysTenant, error) {
	var t SysTenant
	if err := db.First(&t, id).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (s *Service) SetStatus(ctx context.Context, id int64, status int16) error {
	if err := requirePlatform(ctx); err != nil {
		return err
	}
	if id == 0 && status == StatusDisabled {
		return errors.New(81007, 400, "默认租户不能停用")
	}
	if _, err := s.find(s.platformDB(ctx), id); err != nil {
		return err
	}
	err := s.platformDB(ctx).Model(&SysTenant{}).Where("id = ?", id).
		Updates(map[string]any{"status": status, "updated_at": time.Now()}).Error
	s.invalidate(id)
	return err
}

func (s *Service) update(db *gorm.DB, id int64, req UpdateReq) error {
	if _, err := s.find(db, id); err != nil {
		return err
	}
	fields := map[string]any{"updated_at": time.Now()}
	if req.Name != nil {
		fields["name"] = *req.Name
	}
	if req.ExpireAt != nil {
		fields["expire_at"] = *req.ExpireAt
	}
	if len(req.Config) > 0 {
		if !json.Valid(req.Config) {
			return errors.New(10005, 400, "config 不是合法 JSON")
		}
		fields["config"] = req.Config
	}
	err := db.Model(&SysTenant{}).Where("id = ?", id).Updates(fields).Error
	s.invalidate(id)
	return err
}

// Update 平台修改任意租户（含过期时间）。
func (s *Service) Update(ctx context.Context, id int64, req UpdateReq) error {
	if err := requirePlatform(ctx); err != nil {
		return err
	}
	return s.update(s.platformDB(ctx), id, req)
}

// Current 当前登录用户所属租户。
func (s *Service) Current(ctx context.Context) (*SysTenant, error) {
	tid, _ := auth.TenantFromContext(ctx)
	return s.find(s.db.WithContext(ctx), tid)
}

// UpdateCurrentConfig 租户管理员修改本租户名称与配置（不能改过期时间/状态）。
func (s *Service) UpdateCurrentConfig(ctx context.Context, req UpdateReq) error {
	tid, _ := auth.TenantFromContext(ctx)
	req.ExpireAt = nil
	return s.update(s.db.WithContext(ctx), tid, req)
}

func (s *Service) Stats(ctx context.Context, id int64) (*StatsVO, error) {
	if err := requirePlatform(ctx); err != nil {
		return nil, err
	}
	db := s.platformDB(ctx)
	if _, err := s.find(db, id); err != nil {
		return nil, err
	}
	vo := &StatsVO{TenantID: id}
	db.Table("sys_user").Where("tenant_id = ? AND deleted_at IS NULL", id).Count(&vo.Users)
	db.Table("mdm_material").Where("tenant_id = ? AND deleted_at IS NULL", id).Count(&vo.Materials)
	db.Table("pur_purchase_order").Where("tenant_id = ?", id).Count(&vo.PurchaseOrders)
	db.Table("sal_sales_order").Where("tenant_id = ? AND deleted_at IS NULL", id).Count(&vo.SalesOrders)
	return vo, nil
}

func (s *Service) invalidate(id int64) {
	s.mu.Lock()
	delete(s.cache, id)
	s.mu.Unlock()
}

// Check 返回租户可用性：write=true 时过期租户也拒绝。供 TenantGuard 中间件与登录使用。
func (s *Service) Check(ctx context.Context, id int64, write bool) error {
	s.mu.Lock()
	c, ok := s.cache[id]
	s.mu.Unlock()
	if !ok || time.Since(c.at) > cacheTTL {
		t, err := s.find(s.db.WithContext(auth.WithSkipTenant(ctx)), id)
		if err != nil {
			return err
		}
		c = cached{t: *t, at: time.Now()}
		s.mu.Lock()
		s.cache[id] = c
		s.mu.Unlock()
	}
	if c.t.Status != StatusEnabled {
		return ErrTenantDisabled
	}
	if write && c.t.ExpireAt != nil && time.Now().After(*c.t.ExpireAt) {
		return ErrTenantExpired
	}
	return nil
}
