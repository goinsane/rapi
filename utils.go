package rapi

import (
	"reflect"
)

func makePtr(x interface{}) interface{} {
	val := reflect.ValueOf(x)
	if val.Kind() == reflect.Ptr {
		return x
	}
	val2 := reflect.New(val.Type())
	val2.Elem().Set(val)
	return val2.Interface()
}
