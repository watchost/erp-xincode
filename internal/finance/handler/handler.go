// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"net/http"
	"strconv"

	"erp-system/internal/finance/dto"
	"erp-system/internal/finance/service"
	"erp-system/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type FinanceHandler struct {
	financeService *service.FinanceService
}

func NewFinanceHandler(financeService *service.FinanceService) *FinanceHandler {
	return &FinanceHandler{financeService: financeService}
}

func (h *FinanceHandler) ListCostCards(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	productID, _ := strconv.ParseInt(c.Query("product_id"), 10, 64)

	req := dto.CostCardQuery{
		ProductID: productID,
		CostType:  c.Query("cost_type"),
		StartDate: c.Query("start_date"),
		EndDate:   c.Query("end_date"),
		Page:      page,
		PageSize:  pageSize,
	}

	list, total, err := h.financeService.ListCostCards(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

func (h *FinanceHandler) GetCostSummary(c *gin.Context) {
	productID, _ := strconv.ParseInt(c.Query("product_id"), 10, 64)

	list, err := h.financeService.GetCostSummary(c.Request.Context(), productID, c.Query("start_date"), c.Query("end_date"))
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(list))
}

func (h *FinanceHandler) ListAccountEntries(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	req := dto.AccountEntryQuery{
		AccountCode: c.Query("account_code"),
		BizType:     c.Query("biz_type"),
		BizNo:       c.Query("biz_no"),
		StartDate:   c.Query("start_date"),
		EndDate:     c.Query("end_date"),
		Page:        page,
		PageSize:    pageSize,
	}

	list, total, err := h.financeService.ListAccountEntries(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

func (h *FinanceHandler) GetFinancialReport(c *gin.Context) {
	report, err := h.financeService.GetFinancialReport(c.Request.Context(), c.Query("period"))
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(report))
}

func (h *FinanceHandler) ListBudgets(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	year, _ := strconv.Atoi(c.Query("year"))
	month, _ := strconv.Atoi(c.Query("month"))

	list, total, err := h.financeService.ListBudgets(c.Request.Context(), c.Query("budget_type"), year, month, page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

func (h *FinanceHandler) CreateBudget(c *gin.Context) {
	var req dto.CreateBudgetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}

	vo, err := h.financeService.CreateBudget(c.Request.Context(), req.BudgetType, req.Amount, req.Year, req.Month)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}

	c.JSON(http.StatusOK, response.OK(vo))
}

// ----------------- P2 §3.5 应收应付与对账 Handler -----------------

func (h *FinanceHandler) CreateReceivable(c *gin.Context) {
	var req dto.CreateReceivableReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	tenantID := int64(0)
	res, err := h.financeService.CreateReceivable(c.Request.Context(), req, tenantID)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

func (h *FinanceHandler) GetReceivable(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, err := h.financeService.GetReceivable(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

func (h *FinanceHandler) ListReceivables(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	customerID, _ := strconv.ParseInt(c.Query("customer_id"), 10, 64)
	status, _ := strconv.Atoi(c.Query("status"))

	req := dto.ReceivableQuery{
		CustomerID:   customerID,
		ReceivableNo: c.Query("receivable_no"),
		Status:       int16(status),
		Period:       c.Query("period"),
		Page:         page,
		PageSize:     pageSize,
	}

	list, total, err := h.financeService.ListReceivables(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

func (h *FinanceHandler) CreatePayable(c *gin.Context) {
	var req dto.CreatePayableReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	tenantID := int64(0)
	res, err := h.financeService.CreatePayable(c.Request.Context(), req, tenantID)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

func (h *FinanceHandler) GetPayable(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, err := h.financeService.GetPayable(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

func (h *FinanceHandler) ListPayables(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	supplierID, _ := strconv.ParseInt(c.Query("supplier_id"), 10, 64)
	status, _ := strconv.Atoi(c.Query("status"))

	req := dto.PayableQuery{
		SupplierID: supplierID,
		PayableNo:  c.Query("payable_no"),
		Status:     int16(status),
		Period:     c.Query("period"),
		Page:       page,
		PageSize:   pageSize,
	}

	list, total, err := h.financeService.ListPayables(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

func (h *FinanceHandler) CreateReceipt(c *gin.Context) {
	var req dto.CreateReceiptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	userID := c.GetInt64("user_id")
	tenantID := int64(0)
	res, err := h.financeService.CreateReceipt(c.Request.Context(), req, userID, tenantID)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

func (h *FinanceHandler) GetReceipt(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, err := h.financeService.GetReceipt(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

func (h *FinanceHandler) ListReceipts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	customerID, _ := strconv.ParseInt(c.Query("customer_id"), 10, 64)

	req := dto.ReceiptQuery{
		CustomerID: customerID,
		ReceiptNo:  c.Query("receipt_no"),
		Page:       page,
		PageSize:   pageSize,
	}

	list, total, err := h.financeService.ListReceipts(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

func (h *FinanceHandler) CreatePayment(c *gin.Context) {
	var req dto.CreatePaymentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	userID := c.GetInt64("user_id")
	tenantID := int64(0)
	res, err := h.financeService.CreatePayment(c.Request.Context(), req, userID, tenantID)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

func (h *FinanceHandler) GetPayment(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, err := h.financeService.GetPayment(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

func (h *FinanceHandler) ListPayments(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	supplierID, _ := strconv.ParseInt(c.Query("supplier_id"), 10, 64)

	req := dto.PaymentQuery{
		SupplierID: supplierID,
		PaymentNo:  c.Query("payment_no"),
		Page:       page,
		PageSize:   pageSize,
	}

	list, total, err := h.financeService.ListPayments(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

func (h *FinanceHandler) CreateSettlement(c *gin.Context) {
	var req dto.CreateSettlementReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	userID := c.GetInt64("user_id")
	res, err := h.financeService.CreateSettlement(c.Request.Context(), req, userID)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

func (h *FinanceHandler) GetAgingReport(c *gin.Context) {
	customerID, _ := strconv.ParseInt(c.Query("customer_id"), 10, 64)
	supplierID, _ := strconv.ParseInt(c.Query("supplier_id"), 10, 64)

	req := dto.AgingQuery{
		Direction:  c.DefaultQuery("direction", "recv"),
		CustomerID: customerID,
		SupplierID: supplierID,
		AsOfDate:   c.Query("as_of_date"),
	}

	report, err := h.financeService.GetAgingReport(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(report))
}

