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
