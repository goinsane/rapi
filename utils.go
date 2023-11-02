package rapi

import (
	"compress/flate"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

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
		return fmt.Errorf("media type parse error: %w", err)
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

// copyReflectValue copies val and always returns pointer value if val is not pointer.
func copyReflectValue(val reflect.Value) reflect.Value {
	if !val.IsValid() {
		return reflect.ValueOf(new(interface{}))
	}

	var indirectVal reflect.Value
	if val.Kind() != reflect.Ptr {
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

func valuesToStruct(values url.Values, target interface{}) (err error) {
	if target == nil {
		panic(errors.New("target is nil"))
	}

	val := reflect.ValueOf(target)

	var indirectVal reflect.Value
	if val.Kind() != reflect.Ptr {
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

		fieldName := toJSONFieldName(field.Name)
		if v, ok := field.Tag.Lookup("json"); ok {
			s := strings.SplitN(v, ",", 2)[0]
			if s == "-" {
				continue
			}
			s = toJSONFieldName(s)
			if s != "" {
				fieldName = s
			}
		}

		if !values.Has(fieldName) {
			continue
		}
		value := values.Get(fieldName)

		fieldVal := indirectVal.Field(i)

		ifc, kind := fieldVal.Interface(), fieldVal.Kind()

		switch ifc.(type) {
		case string, *string:
			if kind != reflect.Ptr {
				fieldVal.Set(reflect.ValueOf(value))
			} else {
				fieldVal.Set(reflect.ValueOf(&value))
			}
		case []byte, *[]byte, time.Time, *time.Time:
			value = strconv.Quote(value)
			err = json.Unmarshal([]byte(value), fieldVal.Addr().Interface())
			if err != nil {
				return fmt.Errorf("unable to unmarshal field %q value: %w", fieldName, err)
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
	if val.Kind() != reflect.Ptr {
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

		fieldName := toJSONFieldName(field.Name)
		if v, ok := field.Tag.Lookup("json"); ok {
			s := strings.SplitN(v, ",", 2)[0]
			if s == "-" {
				continue
			}
			s = toJSONFieldName(s)
			if s != "" {
				fieldName = s
			}
		}

		fieldVal := indirectVal.Field(i)

		ifc, kind := fieldVal.Interface(), fieldVal.Kind()

		if kind == reflect.Ptr && fieldVal.IsNil() {
			continue
		}

		switch ifc.(type) {
		case string, *string:
			if kind != reflect.Ptr {
				values.Set(fieldName, ifc.(string))
			} else {
				values.Set(fieldName, *ifc.(*string))
			}
		case []byte, *[]byte, time.Time, *time.Time:
			var data []byte
			data, err = json.Marshal(fieldVal.Interface())
			if err != nil {
				return values, fmt.Errorf("unable to marshal field %q value: %w", fieldName, err)
			}
			value := string(data)
			value, _ = strconv.Unquote(value)
			values.Set(fieldName, value)
		default:
			var data []byte
			data, err = json.Marshal(fieldVal.Interface())
			if err != nil {
				return values, fmt.Errorf("unable to marshal field %q value: %w", fieldName, err)
			}
			value := string(data)
			values.Set(fieldName, value)
		}
	}

	return values, nil
}

func toJSONFieldName(s string) string {
	sl := []rune(s)
	result := make([]rune, 0, len(sl))
	for _, r := range sl {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !(unicode.IsPunct(r) && r <= unicode.MaxASCII) {
			continue
		}
		if r == '?' || r == '\\' || r == ',' {
			continue
		}
		result = append(result, r)
	}
	return string(result)
}

func getContentEncoder(w http.ResponseWriter, r *http.Request) (wr io.WriteCloser, err error) {
	w1 := nopWriteCloser{w}

	defer func() {
		if err == nil && w1 != wr {
			w.Header().Del("Content-Length")
		}
	}()

	for _, opt := range ParseHeaderOptions(r.Header.Get("Accept-Encoding")) {
		var q *float64
		if s, ok := opt.Map["q"]; ok {
			if f, e := strconv.ParseFloat(s, 64); e == nil {
				q = &f
			} else {
				return nil, fmt.Errorf("quality level parse error: %w", e)
			}
		}

		switch key := opt.KeyVals[0].Key; key {
		case "gzip":
			level := gzip.DefaultCompression
			if q != nil {
				newLevel := int(*q)
				if gzip.NoCompression <= newLevel && newLevel <= gzip.BestCompression {
					level = newLevel
				} else {
					return nil, fmt.Errorf("invalid quality level %d", newLevel)
				}
			}
			w.Header().Set("Content-Encoding", key)
			wr, _ = gzip.NewWriterLevel(w, level)
			return wr, nil

		case "deflate":
			level := flate.DefaultCompression
			if q != nil {
				newLevel := int(*q)
				if flate.NoCompression <= newLevel && newLevel <= flate.BestCompression {
					level = newLevel
				} else {
					return nil, fmt.Errorf("invalid quality level %d", newLevel)
				}
			}
			w.Header().Set("Content-Encoding", key)
			wr, _ = flate.NewWriter(w, level)
			return wr, nil

		}
	}

	return w1, nil
}
