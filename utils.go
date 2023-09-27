package rapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

func sendJSONResponse(w http.ResponseWriter, out interface{}, code int) error {
	data, err := json.Marshal(out)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return fmt.Errorf("unable to marshal output: %w", err)
	}
	data = append(data, '\n')
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
	w.WriteHeader(code)
	_, err = io.Copy(w, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("unable to write response body: %w", err)
	}
	return nil
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

func strToReflectValue(str string, val reflect.Value) (err error) {
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
		return fmt.Errorf("unknown type %s for value", typ)
	}
	return nil
}

func valuesToStruct(values map[string]string, target interface{}) (err error) {
	if target == nil {
		panic(errors.New("target is nil"))
	}

	val := reflect.ValueOf(target)
	if val.Kind() != reflect.Pointer || val.Elem().Kind() != reflect.Struct {
		panic(errors.New("target must be struct pointer"))
	}
	if val.IsNil() {
		panic(errors.New("target must be non-nil"))
	}

	val = val.Elem()
	typ := val.Type()

	for i, j := 0, typ.NumField(); i < j; i++ {
		field := typ.Field(i)
		if !field.IsExported() || field.Anonymous {
			continue
		}

		fieldVal := val.Field(i)

		var fieldName string
		if v, ok := field.Tag.Lookup("json"); ok {
			fieldName = strings.SplitN(v, ",", 2)[0]
		} else {
			fieldName = strings.ToLower(strings.ReplaceAll(field.Name, "_", ""))
		}
		if fieldName == "-" {
			continue
		}

		if value, ok := values[fieldName]; ok {
			err = strToReflectValue(value, fieldVal)
			if err != nil {
				return fmt.Errorf("unable to set field %q value: %w", fieldName, err)
			}
		}
	}

	return nil
}
