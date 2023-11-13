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

// httpError writes the http error to the http.ResponseWriter according to the request method.
func httpError(r *http.Request, w http.ResponseWriter, error string, code int) {
	if r.Method == http.MethodHead {
		w.WriteHeader(code)
		return
	}
	http.Error(w, error, code)
}

// validateJSONContentType validates whether the content type is 'application/json; charset=utf-8'.
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
func copyReflectValue(val reflect.Value) (copiedVal reflect.Value, err error) {
	if !val.IsValid() {
		return reflect.ValueOf(new(interface{})), nil
	}

	var indirectVal reflect.Value
	if val.Kind() != reflect.Ptr {
		indirectVal = val
	} else {
		if val.IsNil() {
			return reflect.New(val.Type().Elem()), nil
		}
		indirectVal = val.Elem()
	}

	copiedVal = reflect.New(indirectVal.Type())

	data, err := json.Marshal(indirectVal.Interface())
	if err != nil {
		return copiedVal, fmt.Errorf("json marshal error: %w", err)
	}
	err = json.Unmarshal(data, copiedVal.Interface())
	if err != nil {
		return copiedVal, fmt.Errorf("json unmarshal error: %w", err)
	}

	return copiedVal, nil
}

// valuesToStruct puts url.Values to the given struct.
func valuesToStruct(values url.Values, target interface{}) (err error) {
	if target == nil {
		panic(errors.New("target is nil"))
	}

	val := reflect.ValueOf(target)
	typ := val.Type()

	if typ.Kind() != reflect.Ptr || typ.Elem().Kind() != reflect.Struct {
		panic(errors.New("target must be struct pointer"))
	}
	if val.IsNil() {
		panic(errors.New("target struct pointer is nil"))
	}

	indirectVal := val.Elem()
	indirectValType := indirectVal.Type()

	for i, j := 0, indirectValType.NumField(); i < j; i++ {
		field := indirectValType.Field(i)
		if !field.IsExported() || field.Anonymous {
			continue
		}

		fieldName := getJSONFieldName(field)
		if fieldName == "" {
			continue
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

// structToValues returns url.Values containing struct fields as values.
func structToValues(source interface{}) (values url.Values, err error) {
	values = make(url.Values)

	if source == nil {
		return values, nil
	}

	val := reflect.ValueOf(source)
	typ := val.Type()

	if typ.Kind() != reflect.Struct && !(typ.Kind() == reflect.Ptr && typ.Elem().Kind() == reflect.Struct) {
		panic(errors.New("source must be struct or struct pointer or nil"))
	}

	var indirectVal reflect.Value
	if val.Kind() != reflect.Ptr {
		indirectVal = val
	} else {
		if val.IsNil() {
			return values, nil
		}
		indirectVal = val.Elem()
	}
	indirectValType := indirectVal.Type()

	for i, j := 0, indirectValType.NumField(); i < j; i++ {
		field := indirectValType.Field(i)
		if !field.IsExported() || field.Anonymous {
			continue
		}

		fieldName := getJSONFieldName(field)
		if fieldName == "" {
			continue
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

// getJSONFieldName retrieves the JSON field name from the structure field.
func getJSONFieldName(sf reflect.StructField) string {
	fieldName := toJSONFieldName(sf.Name)
	if v, ok := sf.Tag.Lookup("json"); ok {
		s := strings.SplitN(v, ",", 2)[0]
		if s == "-" {
			return ""
		}
		s = toJSONFieldName(s)
		if s != "" {
			fieldName = s
		}
	}
	return fieldName
}

// toJSONFieldName converts the given string to the JSON field name.
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

// getContentEncoder returns the content encoder according to the given http.Request for the given http.ResponseWriter.
func getContentEncoder(w http.ResponseWriter, r *http.Request) (result io.WriteCloser, err error) {
	wc := nopWriteCloser{w}

	defer func() {
		if err == nil && result != wc {
			w.Header().Del("Content-Length")
		}
	}()

	for _, opt := range parseHTTPHeaderOptions(r.Header.Get("Accept-Encoding")) {
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
			result, _ = gzip.NewWriterLevel(w, level)
			return result, nil

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
			result, _ = flate.NewWriter(w, level)
			return result, nil

		}
	}

	return wc, nil
}

// nopWriteCloser implements io.WriteCloser with a no-op Close method wrapping the provided io.Writer.
type nopWriteCloser struct {
	io.Writer
}

// Close is the implementation of io.WriteCloser.
func (nopWriteCloser) Close() error { return nil }

// httpHeaderOption defines single http header option.
type httpHeaderOption struct {
	KeyVals []httpHeaderOptionKeyVal
	Map     map[string]string
}

// httpHeaderOptionKeyVal is a key-value holder for httpHeaderOption.
type httpHeaderOptionKeyVal struct {
	Key string
	Val string
}

// parseHTTPHeaderOptions parses single http header to return list of httpHeaderOption's.
func parseHTTPHeaderOptions(directive string) (options []httpHeaderOption) {
	options = []httpHeaderOption{}

	for _, o := range strings.Split(directive, ",") {
		o = strings.TrimSpace(o)
		option := &httpHeaderOption{
			KeyVals: []httpHeaderOptionKeyVal{},
			Map:     map[string]string{},
		}
		for _, kv := range strings.Split(o, ";") {
			kv = strings.TrimSpace(kv)
			kvs := strings.SplitN(kv, "=", 2)
			optionKeyVal := &httpHeaderOptionKeyVal{
				Key: strings.TrimSpace(kvs[0]),
			}
			if optionKeyVal.Key == "" {
				continue
			}
			if len(kvs) > 1 {
				optionKeyVal.Val = strings.TrimSpace(kvs[1])
			}
			option.KeyVals = append(option.KeyVals, *optionKeyVal)
			if _, ok := option.Map[optionKeyVal.Key]; !ok {
				option.Map[optionKeyVal.Key] = optionKeyVal.Val
			}
		}
		if len(option.KeyVals) <= 0 {
			continue
		}
		options = append(options, *option)
	}

	return
}
