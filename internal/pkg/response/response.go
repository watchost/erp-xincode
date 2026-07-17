// Copyright 2026 zhouhouping. All Rights Reserved.

package response

import (
	"errors"
	"time"

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
