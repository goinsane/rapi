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

	"github.com/goinsane/logng"
)

func sendJSONResponse(logger *logng.Logger, w http.ResponseWriter, out interface{}, code int) {
	data, err := json.Marshal(out)
	if err != nil {
		logger.Errorf("unable to marshal output: %w", err)
		data, _ := json.Marshal(http.StatusText(http.StatusInternalServerError))
		data = append(data, '\n')
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
		w.WriteHeader(http.StatusInternalServerError)
		_, err = io.Copy(w, bytes.NewBuffer(data))
		if err != nil {
			logger.Errorf("unable to write response body: %w", err)
			return
		}
		return
	}
	data = append(data, '\n')
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
	w.WriteHeader(code)
	_, err = io.Copy(w, bytes.NewBuffer(data))
	if err != nil {
		logger.Errorf("unable to write response body: %w", err)
		return
	}
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
	if charset, ok := params["charset"]; ok && charset != "utf-8" {
		return fmt.Errorf("invalid charset %q", charset)
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
