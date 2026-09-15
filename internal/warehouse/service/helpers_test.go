// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	stderrors "errors"

	apperrors "erp-system/internal/pkg/errors"
)

func asAppError(err error, target **apperrors.AppError) bool {
	return stderrors.As(err, target)
}

// isStockShort 判断是否为「库存不足」业务错误（错误码 30001）。
func isStockShort(err error) bool {
	var ae *apperrors.AppError
	return stderrors.As(err, &ae) && ae.Code == 30001
}
