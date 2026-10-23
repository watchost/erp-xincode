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
	recvRepo         repository.ReceivableRepository
	payRepo          repository.PayableRepository
	receiptRepo      repository.ReceiptRepository
	paymentRepo      repository.PaymentRepository
	settleRepo       repository.SettlementRepository
	customerRepo     mdmRepo.CustomerRepository
	supplierRepo     mdmRepo.SupplierRepository
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

func (s *FinanceService) SetReceivableRepository(r repository.ReceivableRepository) {
	s.recvRepo = r
}

func (s *FinanceService) SetPayableRepository(r repository.PayableRepository) {
	s.payRepo = r
}

func (s *FinanceService) SetReceiptRepository(r repository.ReceiptRepository) {
	s.receiptRepo = r
}

func (s *FinanceService) SetPaymentRepository(r repository.PaymentRepository) {
	s.paymentRepo = r
}

func (s *FinanceService) SetSettlementRepository(r repository.SettlementRepository) {
	s.settleRepo = r
}

func (s *FinanceService) SetCustomerRepository(r mdmRepo.CustomerRepository) {
	s.customerRepo = r
}

func (s *FinanceService) SetSupplierRepository(r mdmRepo.SupplierRepository) {
	s.supplierRepo = r
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

func (s *FinanceService) generateNo(ctx context.Context, prefix string) string {
	if s.numbers != nil {
		return s.numbers.Next(ctx, prefix)
	}
	return fmt.Sprintf("%s%s%04d", prefix, time.Now().Format("20060102150405"), time.Now().UnixNano()%10000)
}

// ----------------- P2 §3.5 应收应付与对账核心业务实现 -----------------

func (s *FinanceService) CreateReceivable(ctx context.Context, req dto.CreateReceivableReq, tenantID int64) (*model.FinReceivable, error) {
	if s.customerRepo != nil {
		if _, err := s.customerRepo.FindByID(req.CustomerID); err != nil {
			return nil, errors.New(30002, 404, "客户不存在")
		}
	}
	dueAt, err := time.Parse("2006-01-02", req.DueAt)
	if err != nil {
		return nil, errors.New(40001, 400, "到期日格式错误(YYYY-MM-DD)")
	}
	period := req.Period
	if period == "" {
		period = dueAt.Format("2006-01")
	}

	recvNo := s.generateNo(ctx, "AR")
	sourceType := req.SourceType
	if sourceType == 0 {
		sourceType = 2 // 默认手工录入
	}

	recv := &model.FinReceivable{
		ReceivableNo:   recvNo,
		CustomerID:     req.CustomerID,
		SourceType:     sourceType,
		SourceNo:       req.SourceNo,
		Amount:         decimalx.Money(req.Amount),
		ReceivedAmount: decimal.Zero,
		DueAt:          dueAt,
		Period:         period,
		Status:         model.BillStatusPending,
		Remark:         req.Remark,
		TenantID:       tenantID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.recvRepo.Create(nil, recv); err != nil {
		return nil, err
	}
	return recv, nil
}

func (s *FinanceService) GetReceivable(ctx context.Context, id int64) (*model.FinReceivable, error) {
	recv, err := s.recvRepo.FindByID(id)
	if err != nil {
		return nil, errors.New(40023, 404, "应收单不存在")
	}
	return recv, nil
}

func (s *FinanceService) ListReceivables(ctx context.Context, req dto.ReceivableQuery) ([]*model.FinReceivable, int64, error) {
	return s.recvRepo.List(req.CustomerID, req.ReceivableNo, req.Status, req.Period, req.Page, req.PageSize)
}

func (s *FinanceService) CreatePayable(ctx context.Context, req dto.CreatePayableReq, tenantID int64) (*model.FinPayable, error) {
	if s.supplierRepo != nil {
		if _, err := s.supplierRepo.FindByID(req.SupplierID); err != nil {
			return nil, errors.New(30002, 404, "供应商不存在")
		}
	}
	dueAt, err := time.Parse("2006-01-02", req.DueAt)
	if err != nil {
		return nil, errors.New(40001, 400, "到期日格式错误(YYYY-MM-DD)")
	}
	period := req.Period
	if period == "" {
		period = dueAt.Format("2006-01")
	}

	payNo := s.generateNo(ctx, "AP")
	sourceType := req.SourceType
	if sourceType == 0 {
		sourceType = 2 // 默认手工录入
	}

	pay := &model.FinPayable{
		PayableNo:   payNo,
		SupplierID:  req.SupplierID,
		SourceType:  sourceType,
		SourceNo:    req.SourceNo,
		Amount:      decimalx.Money(req.Amount),
		PaidAmount:  decimal.Zero,
		DueAt:       dueAt,
		Period:      period,
		Status:      model.BillStatusPending,
		Remark:      req.Remark,
		TenantID:    tenantID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.payRepo.Create(nil, pay); err != nil {
		return nil, err
	}
	return pay, nil
}

func (s *FinanceService) GetPayable(ctx context.Context, id int64) (*model.FinPayable, error) {
	pay, err := s.payRepo.FindByID(id)
	if err != nil {
		return nil, errors.New(40024, 404, "应付单不存在")
	}
	return pay, nil
}

func (s *FinanceService) ListPayables(ctx context.Context, req dto.PayableQuery) ([]*model.FinPayable, int64, error) {
	return s.payRepo.List(req.SupplierID, req.PayableNo, req.Status, req.Period, req.Page, req.PageSize)
}

func (s *FinanceService) CreateReceipt(ctx context.Context, req dto.CreateReceiptReq, userID, tenantID int64) (*model.FinReceipt, error) {
	if s.customerRepo != nil {
		if _, err := s.customerRepo.FindByID(req.CustomerID); err != nil {
			return nil, errors.New(30002, 404, "客户不存在")
		}
	}
	rDate, err := time.Parse("2006-01-02", req.ReceiptDate)
	if err != nil {
		return nil, errors.New(40001, 400, "收款日期格式错误(YYYY-MM-DD)")
	}
	pMethod := req.PaymentMethod
	if pMethod == 0 {
		pMethod = 2
	}

	receiptNo := s.generateNo(ctx, "RC")
	receipt := &model.FinReceipt{
		ReceiptNo:     receiptNo,
		CustomerID:    req.CustomerID,
		Amount:        decimalx.Money(req.Amount),
		ReceiptDate:   rDate,
		PaymentMethod: pMethod,
		BankAccount:   req.BankAccount,
		Remark:        req.Remark,
		CreatedBy:     userID,
		TenantID:      tenantID,
		CreatedAt:     time.Now(),
	}

	if err := s.receiptRepo.Create(nil, receipt); err != nil {
		return nil, err
	}
	return receipt, nil
}

func (s *FinanceService) GetReceipt(ctx context.Context, id int64) (*model.FinReceipt, error) {
	receipt, err := s.receiptRepo.FindByID(id)
	if err != nil {
		return nil, errors.New(40025, 404, "收款单不存在")
	}
	return receipt, nil
}

func (s *FinanceService) ListReceipts(ctx context.Context, req dto.ReceiptQuery) ([]*model.FinReceipt, int64, error) {
	return s.receiptRepo.List(req.CustomerID, req.ReceiptNo, req.Page, req.PageSize)
}

func (s *FinanceService) CreatePayment(ctx context.Context, req dto.CreatePaymentReq, userID, tenantID int64) (*model.FinPayment, error) {
	if s.supplierRepo != nil {
		if _, err := s.supplierRepo.FindByID(req.SupplierID); err != nil {
			return nil, errors.New(30002, 404, "供应商不存在")
		}
	}
	pDate, err := time.Parse("2006-01-02", req.PaymentDate)
	if err != nil {
		return nil, errors.New(40001, 400, "付款日期格式错误(YYYY-MM-DD)")
	}
	pMethod := req.PaymentMethod
	if pMethod == 0 {
		pMethod = 2
	}

	paymentNo := s.generateNo(ctx, "PM")
	payment := &model.FinPayment{
		PaymentNo:     paymentNo,
		SupplierID:    req.SupplierID,
		Amount:        decimalx.Money(req.Amount),
		PaymentDate:   pDate,
		PaymentMethod: pMethod,
		BankAccount:   req.BankAccount,
		Remark:        req.Remark,
		CreatedBy:     userID,
		TenantID:      tenantID,
		CreatedAt:     time.Now(),
	}

	if err := s.paymentRepo.Create(nil, payment); err != nil {
		return nil, err
	}
	return payment, nil
}

func (s *FinanceService) GetPayment(ctx context.Context, id int64) (*model.FinPayment, error) {
	payment, err := s.paymentRepo.FindByID(id)
	if err != nil {
		return nil, errors.New(40026, 404, "付款单不存在")
	}
	return payment, nil
}

func (s *FinanceService) ListPayments(ctx context.Context, req dto.PaymentQuery) ([]*model.FinPayment, int64, error) {
	return s.paymentRepo.List(req.SupplierID, req.PaymentNo, req.Page, req.PageSize)
}

func (s *FinanceService) CreateSettlement(ctx context.Context, req dto.CreateSettlementReq, userID int64) (*model.FinSettlement, error) {
	settleAmount := decimalx.Money(req.Amount)
	if !settleAmount.IsPositive() {
		return nil, errors.New(40001, 400, "核销金额必须大于 0")
	}

	var settle *model.FinSettlement
	err := s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		settleNo := s.generateNo(ctx, "ST")

		if req.SettlementType == 1 {
			// 收款核销 (Receipt -> Receivable)
			receipt, err := s.receiptRepo.FindByIDForUpdate(tx, req.BillID)
			if err != nil {
				return errors.New(40025, 404, "收款单不存在")
			}
			usedAmount, err := s.receiptRepo.SumSettledAmountTx(tx, receipt.ID)
			if err != nil {
				return err
			}
			availReceipt := receipt.Amount.Sub(usedAmount)
			if settleAmount.GreaterThan(availReceipt) {
				return errors.New(40027, 400, fmt.Sprintf("收款单剩余可用金额 %v 小于本次核销金额 %v", availReceipt, settleAmount))
			}

			recv, err := s.recvRepo.FindByIDForUpdate(tx, req.TargetID)
			if err != nil {
				return errors.New(40023, 404, "应收单不存在")
			}
			if recv.Status == model.BillStatusSettled || recv.Status == model.BillStatusCancelled {
				return errors.New(40028, 400, "该应收单已结清或已取消")
			}
			unreceived := recv.Amount.Sub(recv.ReceivedAmount)
			if settleAmount.GreaterThan(unreceived) {
				return errors.New(40029, 400, fmt.Sprintf("应收单剩余未结金额 %v 小于本次核销金额 %v", unreceived, settleAmount))
			}

			recv.ReceivedAmount = recv.ReceivedAmount.Add(settleAmount)
			if recv.ReceivedAmount.Equal(recv.Amount) {
				recv.Status = model.BillStatusSettled
			} else {
				recv.Status = model.BillStatusPartial
			}
			recv.UpdatedAt = time.Now()
			if err := s.recvRepo.Update(tx, recv); err != nil {
				return err
			}

			settle = &model.FinSettlement{
				SettlementNo:   settleNo,
				SettlementType: 1,
				BillID:         receipt.ID,
				TargetID:       recv.ID,
				Amount:         settleAmount,
				CreatedBy:      userID,
				CreatedAt:      time.Now(),
			}
			if err := s.settleRepo.Create(tx, settle); err != nil {
				return err
			}

			// 自动生成收款核销财务凭证：借 1002 银行存款 / 贷 1122 应收账款
			return s.PostTx(ctx, tx, PostingInput{
				BizType:     BizTypeReceiptSettlement,
				BizNo:       settleNo,
				Description: fmt.Sprintf("收款核销 %s: 收款单 %s -> 应收单 %s", settleNo, receipt.ReceiptNo, recv.ReceivableNo),
				CreatedBy:   userID,
				Lines: []PostingLine{
					{AccountCode: AccountBankDeposit, Debit: settleAmount},
					{AccountCode: AccountReceivable, Credit: settleAmount},
				},
			})
		} else {
			// 付款核销 (Payment -> Payable)
			payment, err := s.paymentRepo.FindByIDForUpdate(tx, req.BillID)
			if err != nil {
				return errors.New(40026, 404, "付款单不存在")
			}
			usedAmount, err := s.paymentRepo.SumSettledAmountTx(tx, payment.ID)
			if err != nil {
				return err
			}
			availPayment := payment.Amount.Sub(usedAmount)
			if settleAmount.GreaterThan(availPayment) {
				return errors.New(40030, 400, fmt.Sprintf("付款单剩余可用金额 %v 小于本次核销金额 %v", availPayment, settleAmount))
			}

			pay, err := s.payRepo.FindByIDForUpdate(tx, req.TargetID)
			if err != nil {
				return errors.New(40024, 404, "应付单不存在")
			}
			if pay.Status == model.BillStatusSettled || pay.Status == model.BillStatusCancelled {
				return errors.New(40031, 400, "该应付单已结清或已取消")
			}
			unpaid := pay.Amount.Sub(pay.PaidAmount)
			if settleAmount.GreaterThan(unpaid) {
				return errors.New(40032, 400, fmt.Sprintf("应付单剩余未付金额 %v 小于本次核销金额 %v", unpaid, settleAmount))
			}

			pay.PaidAmount = pay.PaidAmount.Add(settleAmount)
			if pay.PaidAmount.Equal(pay.Amount) {
				pay.Status = model.BillStatusSettled
			} else {
				pay.Status = model.BillStatusPartial
			}
			pay.UpdatedAt = time.Now()
			if err := s.payRepo.Update(tx, pay); err != nil {
				return err
			}

			settle = &model.FinSettlement{
				SettlementNo:   settleNo,
				SettlementType: 2,
				BillID:         payment.ID,
				TargetID:       pay.ID,
				Amount:         settleAmount,
				CreatedBy:      userID,
				CreatedAt:      time.Now(),
			}
			if err := s.settleRepo.Create(tx, settle); err != nil {
				return err
			}

			// 自动生成付款核销财务凭证：借 2202 应付账款 / 贷 1002 银行存款
			return s.PostTx(ctx, tx, PostingInput{
				BizType:     BizTypePaymentSettlement,
				BizNo:       settleNo,
				Description: fmt.Sprintf("付款核销 %s: 付款单 %s -> 应付单 %s", settleNo, payment.PaymentNo, pay.PayableNo),
				CreatedBy:   userID,
				Lines: []PostingLine{
					{AccountCode: AccountsPayable, Debit: settleAmount},
					{AccountCode: AccountBankDeposit, Credit: settleAmount},
				},
			})
		}
	})
	if err != nil {
		return nil, err
	}
	return settle, nil
}

func (s *FinanceService) GetAgingReport(ctx context.Context, req dto.AgingQuery) (*dto.AgingReportVO, error) {
	asOf := time.Now().Truncate(24 * time.Hour)
	if req.AsOfDate != "" {
		if t, err := time.Parse("2006-01-02", req.AsOfDate); err == nil {
			asOf = t.Truncate(24 * time.Hour)
		}
	}

	report := &dto.AgingReportVO{
		Direction: req.Direction,
		AsOfDate:  asOf.Format("2006-01-02"),
	}

	partnerMap := make(map[int64]*dto.AgingPartnerItemVO)

	classifyBucket := func(buckets *dto.AgingBucketVO, dueAt time.Time, amount decimal.Decimal) {
		diffDays := int(asOf.Sub(dueAt.Truncate(24 * time.Hour)).Hours() / 24)
		if diffDays <= 0 {
			buckets.NotDue = buckets.NotDue.Add(amount)
		} else if diffDays <= 30 {
			buckets.Days0To30 = buckets.Days0To30.Add(amount)
		} else if diffDays <= 60 {
			buckets.Days31To60 = buckets.Days31To60.Add(amount)
		} else if diffDays <= 90 {
			buckets.Days61To90 = buckets.Days61To90.Add(amount)
		} else if diffDays <= 180 {
			buckets.Days91To180 = buckets.Days91To180.Add(amount)
		} else {
			buckets.Days180Plus = buckets.Days180Plus.Add(amount)
		}
		buckets.TotalAmount = buckets.TotalAmount.Add(amount)
	}

	if req.Direction == "recv" {
		list, err := s.recvRepo.ListUnsettled(req.CustomerID)
		if err != nil {
			return nil, err
		}
		for _, recv := range list {
			remain := recv.Amount.Sub(recv.ReceivedAmount)
			if !remain.IsPositive() {
				continue
			}
			pItem, exists := partnerMap[recv.CustomerID]
			if !exists {
				name := fmt.Sprintf("客户-%d", recv.CustomerID)
				code := fmt.Sprintf("CUS-%d", recv.CustomerID)
				if s.customerRepo != nil {
					if c, _ := s.customerRepo.FindByID(recv.CustomerID); c != nil {
						name = c.Name
						code = c.CustomerCode
					}
				}
				pItem = &dto.AgingPartnerItemVO{
					PartnerID:   recv.CustomerID,
					PartnerCode: code,
					PartnerName: name,
				}
				partnerMap[recv.CustomerID] = pItem
			}
			classifyBucket(&pItem.Buckets, recv.DueAt, remain)
			classifyBucket(&report.Total, recv.DueAt, remain)
		}
	} else {
		list, err := s.payRepo.ListUnsettled(req.SupplierID)
		if err != nil {
			return nil, err
		}
		for _, pay := range list {
			remain := pay.Amount.Sub(pay.PaidAmount)
			if !remain.IsPositive() {
				continue
			}
			pItem, exists := partnerMap[pay.SupplierID]
			if !exists {
				name := fmt.Sprintf("供应商-%d", pay.SupplierID)
				code := fmt.Sprintf("SUP-%d", pay.SupplierID)
				if s.supplierRepo != nil {
					if sup, _ := s.supplierRepo.FindByID(pay.SupplierID); sup != nil {
						name = sup.Name
						code = sup.SupplierCode
					}
				}
				pItem = &dto.AgingPartnerItemVO{
					PartnerID:   pay.SupplierID,
					PartnerCode: code,
					PartnerName: name,
				}
				partnerMap[pay.SupplierID] = pItem
			}
			classifyBucket(&pItem.Buckets, pay.DueAt, remain)
			classifyBucket(&report.Total, pay.DueAt, remain)
		}
	}

	for _, item := range partnerMap {
		report.Items = append(report.Items, *item)
	}

	return report, nil
}

