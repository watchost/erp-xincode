// Copyright 2026 zhouhouping. All Rights Reserved.

package errors

import "fmt"

type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
	Cause   error  `json:"-"`
	HTTP    int    `json:"-"`
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

func New(code, http int, msg string) *AppError {
	return &AppError{Code: code, HTTP: http, Message: msg}
}

func Wrap(err error, code, http int, msg string) *AppError {
	if err == nil {
		return nil
	}
	return &AppError{Code: code, HTTP: http, Message: msg, Cause: err}
}

var codeHTTP = map[int]int{
	0:     200,
	10001: 401,
	10002: 401,
	10003: 401,
	10004: 403,
	10005: 400,
	10100: 404,
	10200: 429,
	10300: 500,
	10400: 500,
	20001: 404,
	20002: 409,
	20003: 409,
	20004: 404,
	20005: 409,
	30001: 409,
	30002: 404,
	30003: 409,
	30004: 409,
	30005: 404,
	30006: 404,
	40001: 404,
	40002: 404,
	40003: 409,
	40004: 409,
	40005: 409,
	50001: 500,
	50002: 409,
	50003: 500,
	60001: 503,
	60002: 504,
	60003: 422,
	60004: 503,
	70001: 401,
	70002: 401,
	70003: 502,
	70004: 400,
	70005: 409,
}

func GetHTTP(code int) int {
	if v, ok := codeHTTP[code]; ok {
		return v
	}
	return 500
}

func Errorf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}
