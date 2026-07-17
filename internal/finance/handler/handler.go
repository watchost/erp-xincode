// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"erp-system/internal/finance/dto"
	"erp-system/internal/finance/service"
	"erp-system/internal/pkg/response"
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
