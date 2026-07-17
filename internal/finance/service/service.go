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
	"erp-system/internal/pkg/db"
)

type FinanceService struct {
	txManager           *db.TxManager
	costCardRepo        repository.CostCardRepository
	accountEntryRepo    repository.AccountEntryRepository
	budgetRepo          repository.BudgetRepository
	materialRepo        mdmRepo.MaterialRepository
}

func NewFinanceService(
	txManager *db.TxManager,
	costCardRepo repository.CostCardRepository,
	accountEntryRepo repository.AccountEntryRepository,
	budgetRepo repository.BudgetRepository,
	materialRepo mdmRepo.MaterialRepository,
) *FinanceService {
	return &FinanceService{
		txManager:        txManager,
		costCardRepo:     costCardRepo,
		accountEntryRepo: accountEntryRepo,
		budgetRepo:       budgetRepo,
		materialRepo:     materialRepo,
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

func (s *FinanceService) GetFinancialReport(ctx context.Context, period string) (*dto.FinancialReportVO, error) {
	endDate := time.Now().Format("2006-01-02")
	if period != "" {
		endDate = period + "-01"
	}

	totalAssets, _ := s.accountEntryRepo.GetBalance("1000", endDate)
	totalLiabilities, _ := s.accountEntryRepo.GetBalance("2000", endDate)

	var totalRevenue, totalCost float64
	_ = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		return tx.Model(&model.FinAccountEntry{}).
			Where("account_code LIKE '5%' AND created_at <= ?", endDate).
			Select("SUM(debit_amount)").Scan(&totalRevenue).Error
	})

	_ = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		return tx.Model(&model.FinAccountEntry{}).
			Where("account_code LIKE '6%' AND created_at <= ?", endDate).
			Select("SUM(debit_amount)").Scan(&totalCost).Error
	})

	grossProfit := totalRevenue - totalCost
	grossMargin := 0.0
	if totalRevenue > 0 {
		grossMargin = (grossProfit / totalRevenue) * 100
	}

	return &dto.FinancialReportVO{
		Period:         period,
		TotalRevenue:   totalRevenue,
		TotalCost:      totalCost,
		GrossProfit:    grossProfit,
		GrossMargin:    grossMargin,
		TotalAssets:    totalAssets,
		TotalLiabilities: totalLiabilities,
		NetAssets:      totalAssets - totalLiabilities,
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
