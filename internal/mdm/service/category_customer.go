// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"

	"erp-system/internal/mdm/dto"
	"erp-system/internal/mdm/model"
	"erp-system/internal/pkg/excel"
	"erp-system/internal/pkg/errors"
	"gorm.io/gorm"
)

// ---- 物料分类树 ----

// ListCategoryTree 返回整棵分类树（按 code 排序，Children 已装配）。
func (s *MDMService) ListCategoryTree(ctx context.Context) ([]dto.CategoryVO, error) {
	all, err := s.categoryRepo.List()
	if err != nil {
		return nil, err
	}

	nodes := make(map[int64]*dto.CategoryVO, len(all))
	for _, c := range all {
		nodes[c.ID] = &dto.CategoryVO{
			ID:        c.ID,
			Code:      c.Code,
			Name:      c.Name,
			ParentID:  c.ParentID,
			Path:      c.Path,
			CreatedAt: c.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	// 递归装配时值拷贝自 nodes，父节点拿不到子节点后续追加的 Children，
	// 因此统一由 assembleCategoryTree 自底向上构建。
	return assembleCategoryTree(all, nodes), nil
}

// assembleCategoryTree 自底向上递归装配，避免值拷贝导致的层级丢失。
func assembleCategoryTree(all []model.MdmMaterialCategory, nodes map[int64]*dto.CategoryVO) []dto.CategoryVO {
	var build func(parentID *int64) []dto.CategoryVO
	build = func(parentID *int64) []dto.CategoryVO {
		var out []dto.CategoryVO
		for _, c := range all {
			match := (parentID == nil && c.ParentID == nil) ||
				(parentID != nil && c.ParentID != nil && *c.ParentID == *parentID)
			if !match {
				continue
			}
			node := *nodes[c.ID]
			node.Children = build(&c.ID)
			if len(node.Children) == 0 {
				node.Children = nil
			}
			out = append(out, node)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
		return out
	}
	return build(nil)
}

func (s *MDMService) CreateCategory(ctx context.Context, c *model.MdmMaterialCategory) error {
	if c.Code == "" || c.Name == "" {
		return errors.New(10005, 400, "分类编码和名称不能为空")
	}
	if existing, _ := s.categoryRepo.FindByCode(c.Code); existing != nil {
		return errors.New(10005, 400, "分类编码已存在")
	}

	parentPath := "/"
	if c.ParentID != nil {
		parent, err := s.categoryRepo.FindByID(*c.ParentID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return errors.New(10100, 404, "父分类不存在")
			}
			return err
		}
		parentPath = parent.Path
	}

	c.Path = "/" // 先落库拿 ID，再回填物化路径
	if err := s.categoryRepo.Create(c); err != nil {
		return err
	}
	c.Path = parentPath + strconv.FormatInt(c.ID, 10) + "/"
	return s.categoryRepo.Update(c)
}

func (s *MDMService) UpdateCategory(ctx context.Context, c *model.MdmMaterialCategory) error {
	if c.ID == 0 {
		return errors.New(10005, 400, "分类 ID 不能为空")
	}
	existing, err := s.categoryRepo.FindByID(c.ID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New(10100, 404, "分类不存在")
		}
		return err
	}
	if c.Code != "" && c.Code != existing.Code {
		if dup, _ := s.categoryRepo.FindByCode(c.Code); dup != nil {
			return errors.New(10005, 400, "分类编码已存在")
		}
		existing.Code = c.Code
	}
	if c.Name != "" {
		existing.Name = c.Name
	}
	// 本期不支持调整父子关系（与 IAM 部门一致），ParentID/Path 不可改。
	return s.categoryRepo.Update(existing)
}

func (s *MDMService) DeleteCategory(ctx context.Context, id int64) error {
	if _, err := s.categoryRepo.FindByID(id); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New(10100, 404, "分类不存在")
		}
		return err
	}
	if n, err := s.categoryRepo.CountChildren(id); err != nil {
		return err
	} else if n > 0 {
		return errors.New(10006, 409, "分类下仍有子分类，不能删除")
	}
	if n, err := s.materialRepo.CountByCategory(id); err != nil {
		return err
	} else if n > 0 {
		return errors.New(10006, 409, "分类下仍有物料，不能删除")
	}
	return s.categoryRepo.Delete(id)
}

// ---- 客户主数据 ----

func (s *MDMService) ListCustomers(ctx context.Context, req dto.CustomerListReq) ([]dto.CustomerVO, int64, error) {
	customers, total, err := s.customerRepo.List(req.CustomerCode, req.Name, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}

	vos := make([]dto.CustomerVO, 0, len(customers))
	for _, c := range customers {
		vos = append(vos, buildCustomerVO(&c))
	}
	return vos, total, nil
}

func (s *MDMService) GetCustomer(ctx context.Context, id int64) (*dto.CustomerVO, error) {
	c, err := s.customerRepo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(10100, 404, "客户不存在")
		}
		return nil, err
	}
	vo := buildCustomerVO(c)
	return &vo, nil
}

func (s *MDMService) CreateCustomer(ctx context.Context, c *model.MdmCustomer) error {
	if c.CustomerCode == "" || c.Name == "" {
		return errors.New(10005, 400, "客户编码和名称不能为空")
	}
	if existing, _ := s.customerRepo.FindByCode(c.CustomerCode); existing != nil {
		return errors.New(10005, 400, "客户编码已存在")
	}
	return s.customerRepo.Create(c)
}

// UpdateCustomer 部分更新客户（P1.5 S5）：先查出现行实体，仅覆盖请求中出现的
// 字段（req 指针非 nil 才覆盖），未传的 credit_limit/level/status 等风控字段
// 保持原值，created_at 也不再被零值覆盖。
func (s *MDMService) UpdateCustomer(ctx context.Context, id int64, req dto.CustomerUpdateReq) error {
	if id == 0 {
		return errors.New(10005, 400, "客户 ID 不能为空")
	}
	existing, err := s.customerRepo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New(10100, 404, "客户不存在")
		}
		return err
	}
	if req.CustomerCode != nil && *req.CustomerCode != existing.CustomerCode {
		if dup, _ := s.customerRepo.FindByCode(*req.CustomerCode); dup != nil {
			return errors.New(10005, 400, "客户编码已存在")
		}
		existing.CustomerCode = *req.CustomerCode
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Contact != nil {
		existing.Contact = *req.Contact
	}
	if req.Phone != nil {
		existing.Phone = *req.Phone
	}
	if req.Address != nil {
		existing.Address = *req.Address
	}
	if req.CreditLimit != nil {
		existing.CreditLimit = *req.CreditLimit
	}
	if req.Level != nil {
		existing.Level = *req.Level
	}
	if req.Status != nil {
		existing.Status = *req.Status
	}
	if req.Attributes != nil {
		attrs, err := json.Marshal(req.Attributes)
		if err != nil {
			return errors.New(10005, 400, "扩展属性序列化失败")
		}
		existing.Attributes = attrs
	}
	return s.customerRepo.Update(existing)
}

func (s *MDMService) DeleteCustomer(ctx context.Context, id int64) error {
	if _, err := s.customerRepo.FindByID(id); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New(10100, 404, "客户不存在")
		}
		return err
	}
	// P2 销售单落地后需在此加"被销售单引用"守卫。
	return s.customerRepo.Delete(id)
}

func buildCustomerVO(c *model.MdmCustomer) dto.CustomerVO {
	var attrs map[string]any
	if c.Attributes != nil {
		json.Unmarshal(c.Attributes, &attrs)
	}
	return dto.CustomerVO{
		ID:           c.ID,
		CustomerCode: c.CustomerCode,
		Name:         c.Name,
		Contact:      c.Contact,
		Phone:        c.Phone,
		Address:      c.Address,
		CreditLimit:  c.CreditLimit,
		Level:        c.Level,
		Attributes:   attrs,
		Status:       c.Status,
		CreatedAt:    c.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func (s *MDMService) ExportCustomersCSV(ctx context.Context, req dto.CustomerListReq) ([]byte, error) {
	req.Page = 1
	req.PageSize = 10000
	list, _, err := s.ListCustomers(ctx, req)
	if err != nil {
		return nil, err
	}
	headers := []string{"客户编码", "客户名称", "联系人", "联系电话", "联系地址", "信用额度"}
	rows := make([][]string, 0, len(list))
	for _, c := range list {
		rows = append(rows, []string{
			c.CustomerCode,
			c.Name,
			c.Contact,
			c.Phone,
			c.Address,
			c.CreditLimit.String(),
		})
	}
	return excel.ExportCSV(headers, rows)
}

func (s *MDMService) ImportCustomersCSV(ctx context.Context, data []byte) (int, error) {
	_, rows, err := excel.ImportCSV(bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	count := 0
	for _, row := range rows {
		if len(row) < 2 || strings.TrimSpace(row[0]) == "" || strings.TrimSpace(row[1]) == "" {
			continue
		}
		c := &model.MdmCustomer{
			CustomerCode: strings.TrimSpace(row[0]),
			Name:         strings.TrimSpace(row[1]),
		}
		if len(row) > 2 {
			c.Contact = strings.TrimSpace(row[2])
		}
		if len(row) > 3 {
			c.Phone = strings.TrimSpace(row[3])
		}
		if len(row) > 4 {
			c.Address = strings.TrimSpace(row[4])
		}
		if len(row) > 5 {
			if d, err := decimal.NewFromString(strings.TrimSpace(row[5])); err == nil {
				c.CreditLimit = d
			}
		}
		if err := s.CreateCustomer(ctx, c); err == nil {
			count++
		}
	}
	return count, nil
}

func (s *MDMService) ExportSuppliersCSV(ctx context.Context, req dto.SupplierListReq) ([]byte, error) {
	req.Page = 1
	req.PageSize = 10000
	list, _, err := s.ListSuppliers(ctx, req)
	if err != nil {
		return nil, err
	}
	headers := []string{"供应商编码", "供应商名称", "联系人"}
	rows := make([][]string, 0, len(list))
	for _, sup := range list {
		rows = append(rows, []string{
			sup.SupplierCode,
			sup.Name,
			sup.Contact,
		})
	}
	return excel.ExportCSV(headers, rows)
}

func (s *MDMService) ImportSuppliersCSV(ctx context.Context, data []byte) (int, error) {
	_, rows, err := excel.ImportCSV(bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	count := 0
	for _, row := range rows {
		if len(row) < 2 || strings.TrimSpace(row[0]) == "" || strings.TrimSpace(row[1]) == "" {
			continue
		}
		sup := &model.MdmSupplier{
			SupplierCode: strings.TrimSpace(row[0]),
			Name:         strings.TrimSpace(row[1]),
		}
		if len(row) > 2 {
			sup.Contact = strings.TrimSpace(row[2])
		}
		if err := s.CreateSupplier(ctx, sup); err == nil {
			count++
		}
	}
	return count, nil
}
