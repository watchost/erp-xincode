// Copyright 2026 zhouhouping. All Rights Reserved.

package decimalx

import (
	"reflect"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/shopspring/decimal"
)

// RegisterValidators 在指定 validator 上注册 decimal 比较标签：
//
//	decgt=N   字段必须 > N
//	decgte=N  字段必须 >= N
//	declte=N  字段必须 <= N
//
// 支持 decimal.Decimal 与 *decimal.Decimal；nil 指针一律放行（可空字段）。
// go-playground/validator 内置的 gt/gte 对 decimal.Decimal（struct kind）会
// panic（"Bad field type"），因此所有 decimal 入参必须改用这组标签。
func RegisterValidators(v *validator.Validate) {
	// callValidationEvenIfNull=true：否则 validator 对 nil 指针字段不调用自定义
	// 校验而直接判失败，可空的 *decimal.Decimal 字段就无法放行 nil。
	must := func(v *validator.Validate, tag string, cmp func(d, param decimal.Decimal) bool) {
		_ = v.RegisterValidation(tag, func(fl validator.FieldLevel) bool {
			d, isNil := fieldDecimal(fl)
			if isNil {
				return true // nil 指针：可空字段放行
			}
			param, err := decimal.NewFromString(fl.Param())
			if err != nil {
				return false
			}
			return cmp(d, param)
		}, true)
	}
	must(v, "decgt", func(d, p decimal.Decimal) bool { return d.Cmp(p) > 0 })
	must(v, "decgte", func(d, p decimal.Decimal) bool { return d.Cmp(p) >= 0 })
	must(v, "declte", func(d, p decimal.Decimal) bool { return d.Cmp(p) <= 0 })
}

// InitGin 把 decimal 校验标签注册到 gin 全局 validator。
// 必须在接收任何 HTTP 请求前调用一次（cmd/server main）。
func InitGin() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		RegisterValidators(v)
	}
}

// fieldDecimal 取出字段上的 decimal，自动解引用指针。
// isNil 为 true 表示 nil 指针（调用方按可空字段放行）。
// 标签误挂在非 decimal 字段上属于编程错误，直接 panic 暴露。
func fieldDecimal(fl validator.FieldLevel) (d decimal.Decimal, isNil bool) {
	v := fl.Field()
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return decimal.Decimal{}, true
		}
		v = v.Elem()
	}
	d, ok := v.Interface().(decimal.Decimal)
	if !ok {
		panic("decimalx: dec* 校验标签只能用于 decimal.Decimal 字段，实际类型 " + v.Type().String())
	}
	return d, false
}
