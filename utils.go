package rapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

func httpError(r *http.Request, w http.ResponseWriter, error string, code int) {
	if r.Method == http.MethodHead {
		w.WriteHeader(code)
		return
	}
	http.Error(w, error, code)
}

func validateJSONContentType(contentType string) error {
	mediatype, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Errorf("unable to parse media type: %w", err)
	}
	switch mediatype {
	case "application/json":
	default:
		return fmt.Errorf("invalid media type %q", mediatype)
	}
	if charset, ok := params["charset"]; ok {
		charset = strings.ToLower(charset)
		switch charset {
		case "utf-8":
		default:
			return fmt.Errorf("invalid charset %q", charset)
		}
	}
	return nil
}

func copyReflectValue(val reflect.Value) reflect.Value {
	if !val.IsValid() {
		return reflect.ValueOf(new(interface{}))
	}

	var indirectVal reflect.Value
	if val.Kind() != reflect.Pointer {
		indirectVal = val
	} else {
		if val.IsNil() {
			panic(errors.New("pointer value is nil"))
		}
		indirectVal = val.Elem()
	}

	copiedVal := reflect.New(indirectVal.Type())

	data, err := json.Marshal(indirectVal.Interface())
	if err != nil {
		panic(fmt.Errorf("unable to marshal value: %w", err))
	}
	err = json.Unmarshal(data, copiedVal.Interface())
	if err != nil {
		panic(fmt.Errorf("unable to unmarshal value data: %w", err))
	}

	return copiedVal
}

/*func strToReflectValue(str string, val reflect.Value) (err error) {
	ifc, typ, kind := val.Interface(), val.Type(), val.Kind()
	switch ifc.(type) {
	case bool, *bool:
		var x bool
		x, err = strconv.ParseBool(str)
		if err != nil {
			return err
		}
		if kind != reflect.Pointer {
			val.Set(reflect.ValueOf(x))
		} else {
			val.Set(reflect.ValueOf(&x))
		}
	case int, *int:
		var x int64
		x, err = strconv.ParseInt(str, 10, 0)
		if err != nil {
			return err
		}
		y := int(x)
		if kind != reflect.Pointer {
			val.Set(reflect.ValueOf(y))
		} else {
			val.Set(reflect.ValueOf(&y))
		}
	case uint, *uint:
		var x uint64
		x, err = strconv.ParseUint(str, 10, 0)
		if err != nil {
			return err
		}
		y := uint(x)
		if kind != reflect.Pointer {
			val.Set(reflect.ValueOf(y))
		} else {
			val.Set(reflect.ValueOf(&y))
		}
	case int64, *int64:
		var x int64
		x, err = strconv.ParseInt(str, 10, 64)
		if err != nil {
			return err
		}
		if kind != reflect.Pointer {
			val.Set(reflect.ValueOf(x))
		} else {
			val.Set(reflect.ValueOf(&x))
		}
	case uint64, *uint64:
		var x uint64
		x, err = strconv.ParseUint(str, 10, 64)
		if err != nil {
			return err
		}
		if kind != reflect.Pointer {
			val.Set(reflect.ValueOf(x))
		} else {
			val.Set(reflect.ValueOf(&x))
		}
	case int32, *int32:
		var x int64
		x, err = strconv.ParseInt(str, 10, 32)
		if err != nil {
			return err
		}
		y := int32(x)
		if kind != reflect.Pointer {
			val.Set(reflect.ValueOf(y))
		} else {
			val.Set(reflect.ValueOf(&y))
		}
	case uint32, *uint32:
		var x uint64
		x, err = strconv.ParseUint(str, 10, 32)
		if err != nil {
			return err
		}
		y := uint32(x)
		if kind != reflect.Pointer {
			val.Set(reflect.ValueOf(y))
		} else {
			val.Set(reflect.ValueOf(&y))
		}
	case string, *string:
		x := str
		if kind != reflect.Pointer {
			val.Set(reflect.ValueOf(x))
		} else {
			val.Set(reflect.ValueOf(&x))
		}
	case float64, *float64:
		var x float64
		x, err = strconv.ParseFloat(str, 64)
		if err != nil {
			return err
		}
		if kind != reflect.Pointer {
			val.Set(reflect.ValueOf(x))
		} else {
			val.Set(reflect.ValueOf(&x))
		}
	case float32, *float32:
		var x float64
		x, err = strconv.ParseFloat(str, 32)
		if err != nil {
			return err
		}
		y := float32(x)
		if kind != reflect.Pointer {
			val.Set(reflect.ValueOf(y))
		} else {
			val.Set(reflect.ValueOf(&y))
		}
	default:
		panic(fmt.Errorf("invalid type %s for value", typ))
	}
	return nil
}*/

func valuesToStruct(values url.Values, target interface{}) (err error) {
	if target == nil {
		panic(errors.New("target is nil"))
	}

	val := reflect.ValueOf(target)

	var indirectVal reflect.Value
	if val.Kind() != reflect.Pointer {
		panic(errors.New("target must be struct pointer"))
	} else {
		if val.IsNil() {
			panic(errors.New("target pointer is nil"))
		}
		indirectVal = val.Elem()
	}

	if indirectVal.Kind() != reflect.Struct {
		panic(errors.New("target must be struct pointer"))
	}

	indirectValType := indirectVal.Type()

	for i, j := 0, indirectValType.NumField(); i < j; i++ {
		field := indirectValType.Field(i)
		if !field.IsExported() || field.Anonymous {
			continue
		}

		fieldVal := indirectVal.Field(i)

		var fieldName string
		if v, ok := field.Tag.Lookup("json"); ok {
			fieldName = strings.SplitN(v, ",", 2)[0]
		} else {
			fieldName = strings.ToLower(strings.ReplaceAll(field.Name, "_", ""))
		}
		if fieldName == "-" {
			continue
		}

		if !values.Has(fieldName) {
			continue
		}
		value := values.Get(fieldName)

		ifc, kind := fieldVal.Interface(), fieldVal.Kind()
		switch ifc.(type) {
		case string, *string:
			x := value
			if kind != reflect.Pointer {
				fieldVal.Set(reflect.ValueOf(x))
			} else {
				fieldVal.Set(reflect.ValueOf(&x))
			}
		default:
			err = json.Unmarshal([]byte(value), fieldVal.Addr().Interface())
			if err != nil {
				return fmt.Errorf("unable to unmarshal field %q value: %w", fieldName, err)
			}
		}
	}

	return nil
}

func structToValues(source interface{}) (values url.Values, err error) {
	if source == nil {
		panic(errors.New("source is nil"))
	}

	val := reflect.ValueOf(source)

	var indirectVal reflect.Value
	if val.Kind() != reflect.Pointer {
		indirectVal = val
	} else {
		if val.IsNil() {
			panic(errors.New("source pointer is nil"))
		}
		indirectVal = val.Elem()
	}

	if indirectVal.Kind() != reflect.Struct {
		panic(errors.New("source must be struct or struct pointer"))
	}

	indirectValType := indirectVal.Type()

	values = make(url.Values)

	for i, j := 0, indirectValType.NumField(); i < j; i++ {
		field := indirectValType.Field(i)
		if !field.IsExported() || field.Anonymous {
			continue
		}

		fieldVal := indirectVal.Field(i)

		var fieldName string
		if v, ok := field.Tag.Lookup("json"); ok {
			fieldName = strings.SplitN(v, ",", 2)[0]
		} else {
			fieldName = strings.ToLower(strings.ReplaceAll(field.Name, "_", ""))
		}
		if fieldName == "-" {
			continue
		}

		ifc, kind := fieldVal.Interface(), fieldVal.Kind()

		if kind == reflect.Pointer && fieldVal.IsNil() {
			continue
		}

		switch ifc.(type) {
		case string, *string:
			if kind != reflect.Pointer {
				values.Set(fieldName, ifc.(string))
			} else {
				values.Set(fieldName, *ifc.(*string))
			}
		default:
			var data []byte
			data, err = json.Marshal(fieldVal.Interface())
			if err != nil {
				return values, fmt.Errorf("unable to marshal field %q value: %w", fieldName, err)
			}
			values.Set(fieldName, string(data))
		}
	}

	return values, nil
}
