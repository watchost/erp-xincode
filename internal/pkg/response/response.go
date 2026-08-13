// Copyright 2026 zhouhouping. All Rights Reserved.

package response

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	appErrors "erp-system/internal/pkg/errors"
)

type Response struct {
	Code      int         `json:"code"`
	Msg       string      `json:"msg"`
	Data      interface{} `json:"data"`
	Timestamp string      `json:"timestamp"`
}

type PageData struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

func OK(data interface{}) Response {
	return Response{
		Code:      0,
		Msg:       "success",
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func Err(code int, msg string) Response {
	return Response{
		Code:      code,
		Msg:       msg,
		Data:      nil,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func Page(list interface{}, total int64, page, pageSize int) Response {
	return OK(PageData{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func FromError(err error) Response {
	var ae *appErrors.AppError
	if errors.As(err, &ae) {
		return Err(ae.Code, ae.Message)
	}
	return Err(10300, "系统内部错误")
}

// Success 写入成功响应，使用 HTTP 200。
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, OK(data))
}

// PageSuccess 写入分页成功响应。
func PageSuccess(c *gin.Context, list interface{}, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, Page(list, total, page, pageSize))
}

// Fail 按错误类型写入真实 HTTP 状态码。
// AppError 中带有 HTTP 字段，未知错误统一为 500。
func Fail(c *gin.Context, err error) {
	var ae *appErrors.AppError
	status := http.StatusInternalServerError
	code := 10300
	msg := "系统内部错误"
	if errors.As(err, &ae) {
		status = ae.HTTP
		if status == 0 {
			status = appErrors.GetHTTP(ae.Code)
		}
		code = ae.Code
		msg = ae.Message
	}
	c.JSON(status, Err(code, msg))
}

// BadRequest 用于参数校验失败等 400 场景。
func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, Err(10005, msg))
}

// Created 写入 201 与响应数据。
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, OK(data))
}
