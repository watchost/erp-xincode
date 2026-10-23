// Copyright 2026 zhouhouping. All Rights Reserved.

package printer

import (
	"net/http"
	"strconv"

	"erp-system/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type PrinterHandler struct{}

func NewPrinterHandler() *PrinterHandler {
	return &PrinterHandler{}
}

// RenderDocument 接收单据数据渲染标准打印 HTML
func (h *PrinterHandler) RenderDocument(c *gin.Context) {
	var data DocumentPrintData
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, "参数错误: "+err.Error()))
		return
	}
	html, err := RenderDocumentHTML(data)
	if err != nil {
		c.JSON(http.StatusOK, response.FromError(err))
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, html)
}

// RenderBarcodeSVG 接收内容生成 SVG 条形码
func (h *PrinterHandler) RenderBarcodeSVG(c *gin.Context) {
	content := c.Query("content")
	if content == "" {
		c.JSON(http.StatusBadRequest, response.Err(10005, "条码内容不能为空"))
		return
	}
	width, _ := strconv.Atoi(c.DefaultQuery("width", "240"))
	height, _ := strconv.Atoi(c.DefaultQuery("height", "60"))

	svg, err := GenerateCode128SVG(content, width, height)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Err(10005, err.Error()))
		return
	}
	c.Header("Content-Type", "image/svg+xml; charset=utf-8")
	c.String(http.StatusOK, svg)
}
