// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	mdmRepo "erp-system/internal/mdm/repository"
	"erp-system/internal/finance/dto"
	"erp-system/internal/finance/model"
	"erp-system/internal/finance/repository"
	"erp-system/internal/pkg/bizno"
	"erp-system/internal/pkg/db"
)

type FinanceService struct {
	txManager           *db.TxManager
	costCardRepo        repository.CostCardRepository
	accountEntryRepo    repository.AccountEntryRepository
	budgetRepo          repository.BudgetRepository
	materialRepo        mdmRepo.MaterialRepository
	numbers             *bizno.Generator
}

func NewFinanceService(
	txManager *db.TxManager,
	costCardRepo repository.CostCardRepository,
	accountEntryRepo repository.AccountEntryRepository,
	budgetRepo repository.BudgetRepository,
	materialRepo mdmRepo.MaterialRepository,
	numbers *bizno.Generator,
) *FinanceService {
	return &FinanceService{
		txManager:        txManager,
		costCardRepo:     costCardRepo,
		accountEntryRepo: accountEntryRepo,
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
			TotalCost:    row.MaterialCost + row.LaborCost + row.OverheadCost,
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
	periodStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	periodEnd := periodStart.AddDate(0, 1, 0) // 次月 1 日，区间左闭右开

	// 资产负债表（时点，期末累计）
	inventoryAssets, err := s.accountEntryRepo.SumBalance("1", nil, &periodEnd)
	if err != nil {
		return nil, err
	}
	wipAssets, err := s.accountEntryRepo.SumBalance("5", nil, &periodEnd) // 生产成本借余=在产品
	if err != nil {
		return nil, err
	}
	totalAssets := inventoryAssets + wipAssets

	liabilityNet, err := s.accountEntryRepo.SumBalance("2", nil, &periodEnd)
	if err != nil {
		return nil, err
	}
	totalLiabilities := -liabilityNet // 负债为贷方余额

	// 利润表（本月期间发生额）
	revenueNet, err := s.accountEntryRepo.SumBalance("60", &periodStart, &periodEnd)
	if err != nil {
		return nil, err
	}
	totalRevenue := -revenueNet // 收入为贷方发生

	totalCost, err := s.accountEntryRepo.SumBalance("64", &periodStart, &periodEnd)
	if err != nil {
		return nil, err
	}

	grossProfit := totalRevenue - totalCost
	grossMargin := 0.0
	if totalRevenue > 0 {
		grossMargin = (grossProfit / totalRevenue) * 100
	}

	return &dto.FinancialReportVO{
		Period:           fmt.Sprintf("%04d-%02d", year, month),
		TotalRevenue:     totalRevenue,
		TotalCost:        totalCost,
		GrossProfit:      grossProfit,
		GrossMargin:      grossMargin,
		TotalAssets:      totalAssets,
		TotalLiabilities: totalLiabilities,
		NetAssets:        totalAssets - totalLiabilities,
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
			RemainingAmount: budget.Amount - budget.UsedAmount,
			Year:            budget.Year,
			Month:           budget.Month,
			Status:          budget.Status,
			StatusDesc:      statusDesc,
			CreatedAt:       budget.CreatedAt.Format(time.RFC3339),
		})
	}

	return vos, total, nil
}

func (s *FinanceService) CreateBudget(ctx context.Context, budgetType string, amount float64, year, month int) (*dto.BudgetVO, error) {
	budgetNo := fmt.Sprintf("BG%v%04d%02d", time.Now().Unix(), year, month)

	var budget model.FinBudget
	err := s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		budget = model.FinBudget{
			BudgetNo:   budgetNo,
			BudgetType: budgetType,
			Amount:     amount,
			UsedAmount: 0,
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
		Amount:          budget.Amount,
		UsedAmount:      budget.UsedAmount,
		RemainingAmount: budget.Amount - budget.UsedAmount,
		Year:            budget.Year,
		Month:           budget.Month,
		Status:          budget.Status,
	}, nil
}
