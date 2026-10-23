// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"erp-system/internal/finance/dto"
	"erp-system/internal/finance/model"
	"erp-system/internal/finance/repository"
	mdmRepo "erp-system/internal/mdm/repository"
	"erp-system/internal/pkg/bizno"
	"erp-system/internal/pkg/db"
	"erp-system/internal/pkg/decimalx"
	"erp-system/internal/pkg/errors"
	"gorm.io/gorm"
)

type FinanceService struct {
	txManager        *db.TxManager
	costCardRepo     repository.CostCardRepository
	accountEntryRepo repository.AccountEntryRepository
	voucherRepo      repository.VoucherRepository
	accountRepo      repository.AccountRepository
	budgetRepo       repository.BudgetRepository
	materialRepo     mdmRepo.MaterialRepository
	numbers          *bizno.Generator
}

func NewFinanceService(
	txManager *db.TxManager,
	costCardRepo repository.CostCardRepository,
	accountEntryRepo repository.AccountEntryRepository,
	voucherRepo repository.VoucherRepository,
	accountRepo repository.AccountRepository,
	budgetRepo repository.BudgetRepository,
	materialRepo mdmRepo.MaterialRepository,
	numbers *bizno.Generator,
) *FinanceService {
	return &FinanceService{
		txManager:        txManager,
		costCardRepo:     costCardRepo,
		accountEntryRepo: accountEntryRepo,
		voucherRepo:      voucherRepo,
		accountRepo:      accountRepo,
		budgetRepo:       budgetRepo,
		materialRepo:     materialRepo,
		numbers:          numbers,
	}
}

func (s *FinanceService) ListCostCards(ctx context.Context, req dto.CostCardQuery) ([]dto.CostCardVO, int64, error) {
	list, total, err := s.costCardRepo.List(req.ProductID, req.CostType, req.StartDate, req.EndDate, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}

	var vos []dto.CostCardVO
	for _, card := range list {
		m, _ := s.materialRepo.FindByID(card.ProductID)
		productCode := ""
		productName := ""
		if m != nil {
			productCode = m.MaterialCode
			productName = m.Name
		}

		costTypeName := ""
		switch card.CostType {
		case "material":
			costTypeName = "材料成本"
		case "labor":
			costTypeName = "人工成本"
		case "overhead":
			costTypeName = "制造费用"
		}

		vos = append(vos, dto.CostCardVO{
			ID:           card.ID,
			ProductID:    card.ProductID,
			ProductCode:  productCode,
			ProductName:  productName,
			CostType:     card.CostType,
			CostTypeName: costTypeName,
			Amount:       card.Amount,
			CostDate:     card.CostDate.Format(time.RFC3339),
			SourceType:   card.SourceType,
			SourceID:     card.SourceID,
		})
	}

	return vos, total, nil
}

func (s *FinanceService) GetCostSummary(ctx context.Context, productID int64, startDate, endDate string) ([]dto.CostSummaryVO, error) {
	rows, err := s.costCardRepo.SummaryByProduct(productID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	var vos []dto.CostSummaryVO
	for _, row := range rows {
		m, _ := s.materialRepo.FindByID(row.ProductID)
		productCode := ""
		productName := ""
		if m != nil {
			productCode = m.MaterialCode
			productName = m.Name
		}

		vos = append(vos, dto.CostSummaryVO{
			ProductID:    row.ProductID,
			ProductCode:  productCode,
			ProductName:  productName,
			MaterialCost: row.MaterialCost,
			LaborCost:    row.LaborCost,
			OverheadCost: row.OverheadCost,
			TotalCost:    row.MaterialCost.Add(row.LaborCost).Add(row.OverheadCost),
		})
	}

	return vos, nil
}

func (s *FinanceService) ListAccountEntries(ctx context.Context, req dto.AccountEntryQuery) ([]dto.AccountEntryVO, int64, error) {
	list, total, err := s.accountEntryRepo.List(req.AccountCode, req.BizType, req.BizNo, req.StartDate, req.EndDate, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}

	var vos []dto.AccountEntryVO
	for _, entry := range list {
		vos = append(vos, dto.AccountEntryVO{
			ID:           entry.ID,
			EntryNo:      entry.EntryNo,
			AccountCode:  entry.AccountCode,
			AccountName:  entry.AccountName,
			DebitAmount:  entry.DebitAmount,
			CreditAmount: entry.CreditAmount,
			Balance:      entry.Balance,
			BizType:      entry.BizType,
			BizNo:        entry.BizNo,
			Description:  entry.Description,
			CreatedAt:    entry.CreatedAt.Format(time.RFC3339),
		})
	}

	return vos, total, nil
}

// GetFinancialReport 生成财务简表。
// 资产负债表为时点数：取期末（< periodEnd）累计余额；利润表为期间数：取本月发生额。
// 科目方向遵循 Chinese GAAP：
//   - 资产类 1xxx 借增、生产成本 5001 借余（在产品，视同存货资产）；
//   - 负债类 2xxx 贷增（SumBalance 借-贷为负，取反得负债余额）；
//   - 收入类 6001/6051（前缀 60）贷方发生，取 -SumBalance；
//   - 营业成本类 6401/6402（前缀 64）借方发生，取 SumBalance。
func (s *FinanceService) GetFinancialReport(ctx context.Context, period string) (*dto.FinancialReportVO, error) {
	now := time.Now()
	year, month := now.Year(), int(now.Month())
	if period != "" {
		if t, err := time.Parse("2006-01", period); err == nil {
			year, month = t.Year(), int(t.Month())
		}
	}
	// 期间区间按凭证主表 period 过滤（YYYY-MM 字典序即时间序），
	// 期末结转凭证因此归属到被结转期间而不是录入当天。
	periodFrom := fmt.Sprintf("%04d-%02d", year, month)
	next := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local).AddDate(0, 1, 0)
	periodTo := next.Format("2006-01") // 次月，区间左闭右开

	// 资产负债表（时点，期末累计）
	inventoryAssets, err := s.accountEntryRepo.SumBalance("1", nil, &periodTo)
	if err != nil {
		return nil, err
	}
	wipAssets, err := s.accountEntryRepo.SumBalance("5", nil, &periodTo) // 生产成本借余=在产品
	if err != nil {
		return nil, err
	}
	totalAssets := inventoryAssets.Add(wipAssets)

	liabilityNet, err := s.accountEntryRepo.SumBalance("2", nil, &periodTo)
	if err != nil {
		return nil, err
	}
	totalLiabilities := liabilityNet.Neg() // 负债为贷方余额

	// 利润表（本月期间发生额）：排除期末结转凭证——结转分录会把
	// 60/64 科目本期发生对冲清零，期间数必须取流量口径（P1.5 H3）。
	revenueNet := decimal.Zero
	if rows, err := s.accountEntryRepo.SumFlowByAccount("60", &periodFrom, &periodTo); err != nil {
		return nil, err
	} else {
		for _, r := range rows {
			revenueNet = revenueNet.Add(r.Net)
		}
	}
	totalRevenue := revenueNet.Neg() // 收入为贷方发生

	totalCost := decimal.Zero
	if rows, err := s.accountEntryRepo.SumFlowByAccount("64", &periodFrom, &periodTo); err != nil {
		return nil, err
	} else {
		for _, r := range rows {
			totalCost = totalCost.Add(r.Net)
		}
	}

	grossProfit := totalRevenue.Sub(totalCost)
	grossMargin := decimal.Zero
	if totalRevenue.IsPositive() {
		// 中间比率保留 6 位小数，×100 后按百分数 2 位归整。
		grossMargin = grossProfit.DivRound(totalRevenue, 6).Mul(decimal.NewFromInt(100)).Round(decimalx.PlacesMoney)
	}

	return &dto.FinancialReportVO{
		Period:           fmt.Sprintf("%04d-%02d", year, month),
		TotalRevenue:     totalRevenue,
		TotalCost:        totalCost,
		GrossProfit:      grossProfit,
		GrossMargin:      grossMargin,
		TotalAssets:      totalAssets,
		TotalLiabilities: totalLiabilities,
		NetAssets:        totalAssets.Sub(totalLiabilities),
	}, nil
}

func (s *FinanceService) ListBudgets(ctx context.Context, budgetType string, year, month, page, pageSize int) ([]dto.BudgetVO, int64, error) {
	list, total, err := s.budgetRepo.List(budgetType, year, month, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var vos []dto.BudgetVO
	for _, budget := range list {
		budgetTypeName := ""
		switch budget.BudgetType {
		case "purchase":
			budgetTypeName = "采购预算"
		case "production":
			budgetTypeName = "生产预算"
		case "marketing":
			budgetTypeName = "营销预算"
		case "admin":
			budgetTypeName = "管理费用"
		}

		statusDesc := ""
		switch budget.Status {
		case 1:
			statusDesc = "预算中"
		case 2:
			statusDesc = "执行中"
		case 3:
			statusDesc = "已完成"
		}

		vos = append(vos, dto.BudgetVO{
			ID:              budget.ID,
			BudgetNo:        budget.BudgetNo,
			BudgetType:      budget.BudgetType,
			BudgetTypeName:  budgetTypeName,
			Amount:          budget.Amount,
			UsedAmount:      budget.UsedAmount,
			RemainingAmount: budget.Amount.Sub(budget.UsedAmount),
			Year:            budget.Year,
			Month:           budget.Month,
			Status:          budget.Status,
			StatusDesc:      statusDesc,
			CreatedAt:       budget.CreatedAt.Format(time.RFC3339),
		})
	}

	return vos, total, nil
}

func (s *FinanceService) CreateBudget(ctx context.Context, budgetType string, amount decimal.Decimal, year, month int) (*dto.BudgetVO, error) {
	budgetNo := fmt.Sprintf("BG%v%04d%02d", time.Now().Unix(), year, month)

	var budget model.FinBudget
	err := s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		// 同类型+年月唯一：预检给出友好提示，DB 唯一约束兜底并发。
		var count int64
		if err := tx.Model(&model.FinBudget{}).
			Where("budget_type = ? AND year = ? AND month = ?", budgetType, year, month).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New(60011, 409, "本期该类型预算已存在，请勿重复创建")
		}
		// 落库前按金额标度归整（NUMERIC(18,2)）。
		budget = model.FinBudget{
			BudgetNo:   budgetNo,
			BudgetType: budgetType,
			Amount:     decimalx.Money(amount),
			Year:       year,
			Month:      month,
			Status:     1,
		}
		return s.budgetRepo.Create(tx, &budget)
	})

	if err != nil {
		return nil, err
	}

	return &dto.BudgetVO{
		ID:              budget.ID,
		BudgetNo:        budget.BudgetNo,
		BudgetType:      budget.BudgetType,
		BudgetTypeName:  budgetTypeName(budget.BudgetType),
		Amount:          budget.Amount,
		UsedAmount:      budget.UsedAmount,
		RemainingAmount: budget.Amount.Sub(budget.UsedAmount),
		Year:            budget.Year,
		Month:           budget.Month,
		Status:          budget.Status,
		StatusDesc:      "预算中",
		CreatedAt:       budget.CreatedAt.Format(time.RFC3339),
	}, nil
}

func budgetTypeName(budgetType string) string {
	switch budgetType {
	case "purchase":
		return "采购预算"
	case "production":
		return "生产预算"
	case "marketing":
		return "营销预算"
	case "admin":
		return "管理费用"
	}
	return ""
}

// ReserveBudgetTx 在调用方事务内原子占用本期（当前年月）指定类型的预算。
// 本期未设置预算时不强制（返回 nil，视为不启用预算控制）；预算余额不足返回错误，
// 由调用方随业务事务一并回滚。条件更新在 DB 层完成，并发审批不会超额占用。
func (s *FinanceService) ReserveBudgetTx(ctx context.Context, tx *gorm.DB, budgetType string, amount decimal.Decimal) error {
	if !amount.IsPositive() {
		return nil
	}
	now := time.Now()
	exists, ok, err := s.budgetRepo.ReserveAtomic(tx, budgetType, now.Year(), int(now.Month()), amount)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if !ok {
		return errors.New(60010, 409, "本期预算余额不足，单据已被拒绝")
	}
	return nil
}

// ReleaseBudgetTx 在调用方事务内释放本期（当前年月）指定类型预算已占用的金额。
// 用于采购订单取消时回退审批时占用的预算。本期未设置预算时为空操作；
// GREATEST 兜底保证占用不会被释放为负。失败随调用方事务一并回滚。
func (s *FinanceService) ReleaseBudgetTx(ctx context.Context, tx *gorm.DB, budgetType string, amount decimal.Decimal) error {
	if !amount.IsPositive() {
		return nil
	}
	now := time.Now()
	_, err := s.budgetRepo.ReleaseAtomic(tx, budgetType, now.Year(), int(now.Month()), amount)
	return err
}
