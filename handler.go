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

	mu              sync.RWMutex
	serveMux        *http.ServeMux
	patternHandlers map[string]*PatternHandler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	serveMux := h.serveMux
	h.mu.RUnlock()
	if serveMux == nil {
		return
	}
	serveMux.ServeHTTP(w, r)
}

func (h *Handler) Handle(pattern string, middleware ...DoFunc) *PatternHandler {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.serveMux == nil {
		h.serveMux = http.NewServeMux()
	}

	if h.patternHandlers == nil {
		h.patternHandlers = make(map[string]*PatternHandler)
	}

	patternHandler := h.patternHandlers[pattern]
	if patternHandler == nil {
		patternHandler = &PatternHandler{
			handler:    h,
			middleware: middleware,
		}
		h.serveMux.Handle(pattern, patternHandler)
		h.patternHandlers[pattern] = patternHandler
	}

	return patternHandler
}

func (h *Handler) onError(err error, r *http.Request) {
	if h.OnError == nil {
		return
	}
	h.OnError(err, r)
}

type PatternHandler struct {
	handler    *Handler
	middleware []DoFunc

	mu             sync.RWMutex
	methodHandlers map[string]*_MethodHandler
}

func (h *PatternHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	ph := h.methodHandlers[r.Method]
	if ph == nil {
		ph = h.methodHandlers[""]
	}
	h.mu.RUnlock()

	if ph == nil {
		h.handler.onError(fmt.Errorf("method %s not allowed", r.Method), r)
		httpError(r, w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	ph.ServeHTTP(w, r)
}

func (h *PatternHandler) Register(method string, in interface{}, do DoFunc, middleware ...DoFunc) *PatternHandler {
	if in == nil {
		panic("input is nil")
	}

	method = strings.ToUpper(method)

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.methodHandlers == nil {
		h.methodHandlers = make(map[string]*_MethodHandler)
	}

	ph := h.methodHandlers[method]
	if ph != nil {
		panic(fmt.Errorf("method %q already registered", method))
	}

	ph = &_MethodHandler{
		patternHandler: h,
		in:             in,
		do:             do,
		middleware:     middleware,
	}
	h.methodHandlers[method] = ph

	return h
}

type _MethodHandler struct {
	patternHandler *PatternHandler
	in             interface{}
	do             DoFunc
	middleware     []DoFunc
}

func (h *_MethodHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
			h.patternHandler.handler.onError(fmt.Errorf("unable to send json response: %w", err), r)
			return
		}
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "" {
		err = validateJSONContentType(contentType)
		if err != nil {
			h.patternHandler.handler.onError(fmt.Errorf("invalid content type %q: %w", contentType, err), r)
			httpError(r, w, "invalid content type", http.StatusBadRequest)
			return
		}
	}

	inVal := reflect.ValueOf(h.in)
	copiedInVal := copyReflectValue(inVal)

	if contentType == "" && (r.Method == http.MethodHead || r.Method == http.MethodGet) {
		err = valuesToStruct(r.URL.Query(), copiedInVal.Interface())
		if err != nil {
			h.patternHandler.handler.onError(fmt.Errorf("invalid query: %w", err), r)
			httpError(r, w, "invalid query", http.StatusBadRequest)
			return
		}
	} else {
		var rd io.Reader = r.Body
		if h.patternHandler.handler.MaxRequestBodySize > 0 {
			rd = io.LimitReader(r.Body, h.patternHandler.handler.MaxRequestBodySize)
		}
		var data []byte
		data, err = io.ReadAll(rd)
		if err != nil {
			h.patternHandler.handler.onError(fmt.Errorf("unable to read request body: %w", err), r)
			httpError(r, w, "unable to read request body", http.StatusBadRequest)
			return
		}
		if len(data) > 0 {
			err = json.Unmarshal(data, copiedInVal.Interface())
			if err != nil {
				h.patternHandler.handler.onError(fmt.Errorf("unable to unmarshal request body: %w", err), r)
				httpError(r, w, "unable to unmarshal request body", http.StatusBadRequest)
				return
			}
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

	for _, m := range h.patternHandler.handler.Middleware {
		m(req, w.Header(), send)
		if sent != 0 {
			return
		}
	}

	for _, m := range h.patternHandler.middleware {
		m(req, w.Header(), send)
		if sent != 0 {
			return
		}
	}

	for _, m := range h.middleware {
		m(req, w.Header(), send)
		if sent != 0 {
			return
		}
	}

	h.do(req, w.Header(), send)
}
