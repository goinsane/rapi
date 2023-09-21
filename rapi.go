package rapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strconv"

	"github.com/goinsane/logng"
)

type DoFunc func(in interface{}, send SendFunc)
type SendFunc func(out interface{}, code int)

type Handler struct {
	Logger *logng.Logger
	In     interface{}
	Do     DoFunc
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
	if indirectInVal.Kind() != reflect.Struct {
		panic("input must be struct or struct pointer")
	}
	copiedInVal := reflect.New(indirectInVal.Type())
	copiedInVal.Elem().Set(indirectInVal)

	send := func(out interface{}, code int) {
		data, err := json.Marshal(out)
		if err != nil {
			h.Logger.Errorf("unable to marshal response body to json: %w", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		data = append(data, '\n')
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
		w.WriteHeader(code)
		_, err = io.Copy(w, bytes.NewBuffer(data))
		if err != nil {
			h.Logger.Errorf("unable to write data: %w", err)
			return
		}
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		h.Logger.Errorf("unable to read request body: %w", err)
		send(&errorResponse{Error: "unable to read request body"}, http.StatusBadRequest)
		return
	}

	err = json.Unmarshal(data, copiedInVal.Interface())
	if err != nil {
		h.Logger.Errorf("unable to unmarshal request body from json: %w", err)
		send(&errorResponse{Error: "unable to unmarshal request body from json"}, http.StatusBadRequest)
		return
	}

	var in interface{}
	if !isInValPtr {
		in = copiedInVal.Elem().Interface()
	} else {
		in = copiedInVal.Interface()
	}

	h.Do(in, send)
}
