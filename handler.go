package rapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
)

type Handler struct {
	Middleware         []DoFunc
	MaxRequestBodySize int64
	OnError            func(error, *http.Request)

	handlersMu sync.RWMutex
	handlers   map[string]*_PureHandler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handlersMu.RLock()
	ph := h.handlers[r.Method]
	if ph == nil {
		ph = h.handlers[""]
	}
	h.handlersMu.RUnlock()

	if ph == nil {
		h.onError(fmt.Errorf("method %s not allowed", r.Method), r)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	ph.ServeHTTP(w, r)
}

func (h *Handler) Register(method string, in interface{}, do DoFunc, middleware ...DoFunc) *Handler {
	if in == nil {
		panic("input is nil")
	}
	method = strings.ToUpper(method)
	h.handlersMu.Lock()
	defer h.handlersMu.Unlock()
	if h.handlers == nil {
		h.handlers = make(map[string]*_PureHandler)
	}
	ph := h.handlers[method]
	if ph != nil {
		panic(fmt.Errorf("method %q already registered", method))
	}
	ph = &_PureHandler{
		Handler:    h,
		In:         in,
		Do:         do,
		Middleware: middleware,
	}
	h.handlers[method] = ph
	return h
}

func (h *Handler) onError(err error, r *http.Request) {
	if h.OnError == nil {
		return
	}
	h.OnError(err, r)
}

type _PureHandler struct {
	Handler    *Handler
	In         interface{}
	Do         DoFunc
	Middleware []DoFunc
}

func (h *_PureHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(r.Body)

	var sent int32
	send := func(out interface{}, code int) {
		if !atomic.CompareAndSwapInt32(&sent, 0, 1) {
			panic(errors.New("already sent"))
		}
		if r.Method == http.MethodHead {
			w.WriteHeader(code)
			return
		}
		err = sendJSONResponse(w, out, code)
		if err != nil {
			h.Handler.onError(fmt.Errorf("unable to send json response: %w", err), r)
			return
		}
	}

	inVal := reflect.ValueOf(h.In)
	copiedInVal := copyReflectValue(inVal)

	contentType := r.Header.Get("Content-Type")
	if contentType != "" {
		err = validateJSONContentType(contentType)
		if err != nil {
			h.Handler.onError(fmt.Errorf("invalid content type %q: %w", contentType, err), r)
			http.Error(w, "invalid content type", http.StatusBadRequest)
			return
		}
	}

	if contentType == "" && (r.Method == http.MethodHead || r.Method == http.MethodGet) {
		values := map[string]string{}
		for k := range r.URL.Query() {
			values[k] = r.URL.Query().Get(k)
		}
		err = valuesToStruct(values, copiedInVal.Interface())
		if err != nil {
			h.Handler.onError(fmt.Errorf("invalid query: %w", err), r)
			http.Error(w, "invalid query", http.StatusBadRequest)
			return
		}
	} else {
		var rd io.Reader = r.Body
		if h.Handler.MaxRequestBodySize > 0 {
			rd = io.LimitReader(r.Body, h.Handler.MaxRequestBodySize)
		}
		var data []byte
		data, err = io.ReadAll(rd)
		if err != nil {
			h.Handler.onError(fmt.Errorf("unable to read request body: %w", err), r)
			http.Error(w, "unable to read request body", http.StatusBadRequest)
			return
		}

		err = json.Unmarshal(data, copiedInVal.Interface())
		if err != nil {
			h.Handler.onError(fmt.Errorf("unable to unmarshal request body: %w", err), r)
			http.Error(w, "unable to unmarshal request body", http.StatusBadRequest)
			return
		}
	}

	var in interface{}
	if inVal.Kind() == reflect.Pointer {
		in = copiedInVal.Interface()
	} else {
		in = copiedInVal.Elem().Interface()
	}

	req := &Request{
		Request: r,
		In:      in,
	}

	for _, m := range h.Handler.Middleware {
		m(req, w.Header(), send)
		if sent != 0 {
			return
		}
	}

	for _, m := range h.Middleware {
		m(req, w.Header(), send)
		if sent != 0 {
			return
		}
	}

	h.Do(req, w.Header(), send)
}
