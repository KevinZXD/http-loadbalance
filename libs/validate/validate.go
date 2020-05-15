package validate

import (
	"reflect"
	"strconv"

	"github.com/go-playground/validator/v10"
)

var (
	// Validator 自定义校验器，添加default校验
	//
	// 详见 https://godoc.org/gopkg.in/go-playground/validator.v10
	Validator = validator.New()
)

func init() {
	err := Validator.RegisterValidation("default", defaultFunc)
	if err != nil {
		panic("validator register 'default' tag got error: " + err.Error())
	}
}

// defaultFunc 设置默认值校验器，支持int,float,string等
//
// 当数值=0，或字符串为空时，将赋值为tag中default对应的值
// 如 Age int `default=20` 当结构体中的Age值为0时，该校验器会变更为20
func defaultFunc(fl validator.FieldLevel) bool {
	field := fl.Field()
	kind := field.Type().Kind()
	switch {
	case kind == reflect.String:
		if field.String() == "" {
			field.SetString(fl.Param())
		}
	case reflect.Int <= kind && kind <= reflect.Int64:
		if field.Int() == 0 {
			z, e := strconv.Atoi(fl.Param())
			if e != nil {
				return false
			}
			field.SetInt(int64(z))
		}
	case reflect.Uint <= kind && kind <= reflect.Uint64:
		if field.Uint() == 0 {
			z, e := strconv.Atoi(fl.Param())
			if e != nil {
				return false
			}
			field.SetUint(uint64(z))
		}
	case reflect.Float32 <= kind && kind <= reflect.Float64:
		if field.Float() == 0 {
			z, e := strconv.ParseFloat(fl.Param(), 64)
			if e != nil {
				return false
			}
			field.SetFloat(z)
		}
	}
	return true
}
