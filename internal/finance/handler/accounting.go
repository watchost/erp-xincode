// Copyright 2026 zhouhouping. All Rights Reserved.

package handler

import (
	"net/http"
	"strconv"

	"erp-system/internal/finance/dto"
	"erp-system/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *FinanceHandler) ListAccountTree(c *gin.Context) {
	tree, err := h.financeService.ListAccountTree(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(tree))
}

func (h *FinanceHandler) CreateAccount(c *gin.Context) {
	var req dto.AccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	vo, err := h.financeService.CreateAccount(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(vo))
}

func (h *FinanceHandler) UpdateAccount(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req dto.AccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	if err := h.financeService.UpdateAccount(c.Request.Context(), id, req); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *FinanceHandler) DeleteAccount(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.financeService.DeleteAccount(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *FinanceHandler) ListVouchers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	req := dto.VoucherQuery{
		Period:   c.Query("period"),
		BizType:  c.Query("biz_type"),
		Page:     page,
		PageSize: pageSize,
	}
	list, total, err := h.financeService.ListVouchers(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.Page(list, total, page, pageSize))
}

func (h *FinanceHandler) GetVoucher(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	vo, err := h.financeService.GetVoucher(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(vo))
}

func (h *FinanceHandler) ClosePeriod(c *gin.Context) {
	var req dto.PeriodCloseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	if err := h.financeService.ClosePeriod(c.Request.Context(), req.Period); err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *FinanceHandler) GetIncomeStatement(c *gin.Context) {
	vo, err := h.financeService.GetIncomeStatement(c.Request.Context(), c.Query("period"))
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(vo))
}

func (h *FinanceHandler) GetCashFlow(c *gin.Context) {
	vo, err := h.financeService.GetCashFlow(c.Request.Context(), c.Query("period"))
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.JSON(http.StatusOK, response.OK(vo))
}
