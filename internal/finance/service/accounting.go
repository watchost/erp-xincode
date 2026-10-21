// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"erp-system/internal/finance/dto"
	"erp-system/internal/finance/model"
	"erp-system/internal/pkg/auth"
	"erp-system/internal/pkg/errors"
)

// 期末结转相关科目。
const (
	AccountRetainedEarnings = "4103" // 本年利润（权益类，贷增借减）
	BizTypePeriodClose      = "period_close"
)

// ---- 会计科目 ----

// ListAccountTree 返回科目树（按 code 排序）。
func (s *FinanceService) ListAccountTree(ctx context.Context) ([]dto.AccountVO, error) {
	all, err := s.accountRepo.List()
	if err != nil {
		return nil, err
	}

	var build func(parentID *int64) []dto.AccountVO
	build = func(parentID *int64) []dto.AccountVO {
		var out []dto.AccountVO
		for _, a := range all {
			match := (parentID == nil && a.ParentID == nil) ||
				(parentID != nil && a.ParentID != nil && *a.ParentID == *parentID)
			if !match {
				continue
			}
			node := dto.AccountVO{
				ID:        a.ID,
				Code:      a.Code,
				Name:      a.Name,
				ParentID:  a.ParentID,
				Level:     a.Level,
				Category:  a.Category,
				Direction: a.Direction,
				IsLeaf:    a.IsLeaf,
				Children:  build(&a.ID),
			}
			if len(node.Children) == 0 {
				node.Children = nil
			}
			out = append(out, node)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
		return out
	}
	return build(nil), nil
}

func (s *FinanceService) CreateAccount(ctx context.Context, req dto.AccountReq) (*dto.AccountVO, error) {
	if existing, _ := s.accountRepo.FindByCode(req.Code); existing != nil {
		return nil, errors.New(60007, 400, "科目编码已存在")
	}

	level := 1
	if req.ParentID != nil {
		parent, err := s.accountRepo.FindByID(*req.ParentID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, errors.New(60008, 404, "父科目不存在")
			}
			return nil, err
		}
		if parent.Category != req.Category {
			return nil, errors.New(60007, 400, "子科目类别必须与父科目一致")
		}
		level = parent.Level + 1
		// 父科目有了子级后不再是末级
		if parent.IsLeaf {
			parent.IsLeaf = false
			if err := s.accountRepo.Update(parent); err != nil {
				return nil, err
			}
		}
	}

	a := model.FinAccount{
		Code:      req.Code,
		Name:      req.Name,
		ParentID:  req.ParentID,
		Level:     level,
		Category:  req.Category,
		Direction: req.Direction,
		IsLeaf:    true,
	}
	if err := s.accountRepo.Create(&a); err != nil {
		return nil, err
	}
	return &dto.AccountVO{
		ID: a.ID, Code: a.Code, Name: a.Name, ParentID: a.ParentID,
		Level: a.Level, Category: a.Category, Direction: a.Direction, IsLeaf: a.IsLeaf,
	}, nil
}

func (s *FinanceService) UpdateAccount(ctx context.Context, id int64, req dto.AccountReq) error {
	a, err := s.accountRepo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New(60008, 404, "科目不存在")
		}
		return err
	}
	if req.Code != "" && req.Code != a.Code {
		if dup, _ := s.accountRepo.FindByCode(req.Code); dup != nil {
			return errors.New(60007, 400, "科目编码已存在")
		}
		// 已有分录的科目不允许改编码（分录按编码记账）
		if used, err := s.accountEntryRepo.ExistsByAccount(a.Code); err != nil {
			return err
		} else if used {
			return errors.New(60009, 409, "科目已有分录发生，不能修改编码")
		}
		a.Code = req.Code
	}
	if req.Name != "" {
		a.Name = req.Name
	}
	// 类别/方向/父级一经使用不可调整，本期不开放。
	return s.accountRepo.Update(a)
}

func (s *FinanceService) DeleteAccount(ctx context.Context, id int64) error {
	a, err := s.accountRepo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New(60008, 404, "科目不存在")
		}
		return err
	}
	if n, err := s.accountRepo.CountChildren(id); err != nil {
		return err
	} else if n > 0 {
		return errors.New(60009, 409, "科目下仍有子科目，不能删除")
	}
	if used, err := s.accountEntryRepo.ExistsByAccount(a.Code); err != nil {
		return err
	} else if used {
		return errors.New(60009, 409, "科目已有分录发生，不能删除")
	}
	return s.accountRepo.Delete(id)
}

// ---- 凭证查询 ----

func (s *FinanceService) ListVouchers(ctx context.Context, req dto.VoucherQuery) ([]dto.VoucherVO, int64, error) {
	list, total, err := s.voucherRepo.List(req.Period, req.BizType, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}
	vos := make([]dto.VoucherVO, 0, len(list))
	for _, v := range list {
		vos = append(vos, buildVoucherVO(&v, nil))
	}
	return vos, total, nil
}

func (s *FinanceService) GetVoucher(ctx context.Context, id int64) (*dto.VoucherVO, error) {
	v, err := s.voucherRepo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(60008, 404, "凭证不存在")
		}
		return nil, err
	}
	entries, err := s.accountEntryRepo.ListByVoucher(v.ID)
	if err != nil {
		return nil, err
	}
	vos := make([]dto.AccountEntryVO, 0, len(entries))
	for _, e := range entries {
		vos = append(vos, dto.AccountEntryVO{
			ID:           e.ID,
			EntryNo:      e.EntryNo,
			AccountCode:  e.AccountCode,
			AccountName:  e.AccountName,
			DebitAmount:  e.DebitAmount,
			CreditAmount: e.CreditAmount,
			Balance:      e.Balance,
			BizType:      e.BizType,
			BizNo:        e.BizNo,
			Description:  e.Description,
			CreatedAt:    e.CreatedAt.Format(time.RFC3339),
		})
	}
	vo := buildVoucherVO(v, vos)
	return &vo, nil
}

func buildVoucherVO(v *model.FinVoucher, entries []dto.AccountEntryVO) dto.VoucherVO {
	return dto.VoucherVO{
		ID:          v.ID,
		VoucherNo:   v.VoucherNo,
		Period:      v.Period,
		Summary:     v.Summary,
		BizType:     v.BizType,
		BizNo:       v.BizNo,
		DebitTotal:  v.DebitTotal,
		CreditTotal: v.CreditTotal,
		PostedAt:    v.PostedAt.Format(time.RFC3339),
		CreatedBy:   v.CreatedBy,
		Entries:     entries,
	}
}

// ---- 期末结转 ----

// ClosePeriod 期末结转：把指定期间损益类科目（5 收入贷方 / 6 费用借方）净额
// 结转到 4103 本年利润。收入：借 6xxx? 不——收入类结平是 借 5xxx收入 / 贷 4103；
// 费用类结平是 借 4103 / 贷 6xxx费用。一个期间只能结转一次（uk 索引兜底）。
func (s *FinanceService) ClosePeriod(ctx context.Context, period string) error {
	if _, err := time.Parse("2006-01", period); err != nil {
		return errors.New(10005, 400, "期间格式应为 YYYY-MM")
	}
	if exists, err := s.voucherRepo.ExistsPeriodClose(period); err != nil {
		return err
	} else if exists {
		return errors.New(60010, 409, "该期间已结转，不能重复结转")
	}

	periodTo := nextPeriod(period)

	revenues, err := s.accountEntryRepo.SumByAccount("60", &period, &periodTo)
	if err != nil {
		return err
	}
	expenses, err := s.accountEntryRepo.SumByAccount("64", &period, &periodTo)
	if err != nil {
		return err
	}
	adminExpenses, err := s.accountEntryRepo.SumByAccount("66", &period, &periodTo)
	if err != nil {
		return err
	}
	expenses = append(expenses, adminExpenses...)

	var lines []PostingLine
	profit := decimal.Zero
	for _, r := range revenues {
		// 收入类为贷方净额（net 为负），结平借记本科目
		amt := r.Net.Neg()
		if !amt.IsPositive() {
			continue
		}
		lines = append(lines,
			PostingLine{AccountCode: r.AccountCode, AccountName: r.AccountName, Debit: amt},
			PostingLine{AccountCode: AccountRetainedEarnings, AccountName: "本年利润", Credit: amt},
		)
		profit = profit.Add(amt)
	}
	for _, e := range expenses {
		// 费用类为借方净额（net 为正），结平贷记本科目
		amt := e.Net
		if !amt.IsPositive() {
			continue
		}
		lines = append(lines,
			PostingLine{AccountCode: AccountRetainedEarnings, AccountName: "本年利润", Debit: amt},
			PostingLine{AccountCode: e.AccountCode, AccountName: e.AccountName, Credit: amt},
		)
		profit = profit.Sub(amt)
	}

	if len(lines) == 0 {
		return errors.New(60011, 409, "本期无损益发生，无需结转")
	}

	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		return s.PostTx(ctx, tx, PostingInput{
			BizType:     BizTypePeriodClose,
			BizNo:       period,
			Description: fmt.Sprintf("%s 期末损益结转（净利润 %s）", period, profit.String()),
			Period:      period, // 凭证归属被结转期间，报表取数不受影响月干扰
			CreatedBy:   auth.UserIDFromContext(ctx),
			Lines:       lines,
		})
	})
}

// nextPeriod 返回 "YYYY-MM" 的下一期间（用于左闭右开区间）。
func nextPeriod(period string) string {
	t, _ := time.Parse("2006-01", period)
	return t.AddDate(0, 1, 0).Format("2006-01")
}

// ---- 利润表 / 现金流量 ----

// GetIncomeStatement 利润表：按科目列示本期收入（5xxx 贷方净额）与费用（6xxx 借方净额）。
func (s *FinanceService) GetIncomeStatement(ctx context.Context, period string) (*dto.IncomeStatementVO, error) {
	if period == "" {
		period = time.Now().Format("2006-01")
	}
	periodTo := nextPeriod(period)

	revenueRows, err := s.accountEntryRepo.SumByAccount("60", &period, &periodTo)
	if err != nil {
		return nil, err
	}
	expenseRows, err := s.accountEntryRepo.SumByAccount("64", &period, &periodTo)
	if err != nil {
		return nil, err
	}
	// 66xx 期间费用同样属于费用类
	adminRows, err := s.accountEntryRepo.SumByAccount("66", &period, &periodTo)
	if err != nil {
		return nil, err
	}
	expenseRows = append(expenseRows, adminRows...)

	vo := &dto.IncomeStatementVO{Period: period}
	for _, r := range revenueRows {
		amt := r.Net.Neg() // 收入贷方净额取正
		vo.Revenue = append(vo.Revenue, dto.IncomeStatementLineVO{AccountCode: r.AccountCode, AccountName: r.AccountName, Amount: amt})
		vo.TotalRevenue = vo.TotalRevenue.Add(amt)
	}
	for _, e := range expenseRows {
		vo.Expense = append(vo.Expense, dto.IncomeStatementLineVO{AccountCode: e.AccountCode, AccountName: e.AccountName, Amount: e.Net})
		vo.TotalExpense = vo.TotalExpense.Add(e.Net)
	}
	vo.NetProfit = vo.TotalRevenue.Sub(vo.TotalExpense)
	return vo, nil
}

// GetCashFlow 现金流量简表：按业务类型汇总现金类科目（1001/1002）借贷发生。
// 当前业务事件均挂应收应付、不经现金科目，故多数期间为 0——
// 待 P2 收付款单落地后自动有数。
func (s *FinanceService) GetCashFlow(ctx context.Context, period string) (*dto.CashFlowVO, error) {
	if period == "" {
		period = time.Now().Format("2006-01")
	}
	periodTo := nextPeriod(period)

	rows, err := s.accountEntryRepo.CashFlowByBizType(&period, &periodTo)
	if err != nil {
		return nil, err
	}
	vo := &dto.CashFlowVO{Period: period}
	for _, r := range rows {
		vo.Lines = append(vo.Lines, dto.CashFlowLineVO{
			BizType: r.BizType,
			Inflow:  r.Inflow,
			Outflow: r.Outflow,
			Net:     r.Inflow.Sub(r.Outflow),
		})
		vo.TotalInflow = vo.TotalInflow.Add(r.Inflow)
		vo.TotalOutflow = vo.TotalOutflow.Add(r.Outflow)
	}
	vo.NetCashFlow = vo.TotalInflow.Sub(vo.TotalOutflow)
	return vo, nil
}
