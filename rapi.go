package rapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"sync/atomic"

	"github.com/goinsane/logng"
)

type DoFunc func(in interface{}, header http.Header, send SendFunc)
type SendFunc func(out interface{}, header http.Header, code int)

type Handler struct {
	Logger             *logng.Logger
	In                 interface{}
	Do                 DoFunc
	MaxRequestBodySize int64
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(r.Body)

	inVal := reflect.ValueOf(h.In)
	isInValPtr := inVal.Kind() == reflect.Pointer
	var indirectInVal reflect.Value
	if !isInValPtr {
		indirectInVal = inVal
	} else {
		indirectInVal = inVal.Elem()
	}
	switch indirectInVal.Kind() {
	case reflect.Slice, reflect.Map:
		isInValPtr = false
	}
	copiedInVal := reflect.New(indirectInVal.Type())
	copiedInVal.Elem().Set(indirectInVal)

	var sent int32
	send := func(out interface{}, header http.Header, code int) {
		if !atomic.CompareAndSwapInt32(&sent, 0, 1) {
			panic("already sent")
		}
		data, err := json.Marshal(out)
		if err != nil {
			h.Logger.Errorf("unable to marshal response body to json: %w", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		data = append(data, '\n')
		for key, val := range header {
			w.Header()[key] = val
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
		w.WriteHeader(code)
		_, err = io.Copy(w, bytes.NewBuffer(data))
		if err != nil {
			h.Logger.Errorf("unable to write data: %w", err)
			return
		}
	}

	var rd io.Reader = r.Body
	if h.MaxRequestBodySize > 0 {
		rd = io.LimitReader(r.Body, h.MaxRequestBodySize)
	}
	data, err := io.ReadAll(rd)
	if err != nil {
		h.Logger.Errorf("unable to read request body: %w", err)
		send(&ErrorResponse{Error: "unable to read request body"}, nil, http.StatusBadRequest)
		return
	}

	err = json.Unmarshal(data, copiedInVal.Interface())
	if err != nil {
		h.Logger.Errorf("unable to unmarshal request body from json: %w", err)
		send(&ErrorResponse{Error: "unable to unmarshal request body from json"}, nil, http.StatusBadRequest)
		return
	}

	var in interface{}
	if !isInValPtr {
		in = copiedInVal.Elem().Interface()
	} else {
		in = copiedInVal.Interface()
	}

	h.Do(in, r.Header.Clone(), send)
}
