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
	options  *handlerOptions
	serveMux *http.ServeMux
}

func NewHandler(opts ...HandlerOption) (h *Handler) {
	h = &Handler{
		options:  newHandlerOptions(),
		serveMux: http.NewServeMux(),
	}
	newJoinHandlerOption(opts...).apply(h.options)
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.serveMux.ServeHTTP(w, r)
}

func (h *Handler) Handle(pattern string, opts ...HandlerOption) *PatternHandler {
	ph := newPatternHandler(h.options, opts...)
	h.serveMux.Handle(pattern, ph)
	return ph
}

type PatternHandler struct {
	mu             sync.RWMutex
	options        *handlerOptions
	methodHandlers map[string]*methodHandler
}

func newPatternHandler(options *handlerOptions, opts ...HandlerOption) (h *PatternHandler) {
	h = &PatternHandler{
		options:        options.Clone(),
		methodHandlers: make(map[string]*methodHandler),
	}
	newJoinHandlerOption(opts...).apply(h.options)
	return h
}

func (h *PatternHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	mh := h.methodHandlers[r.Method]
	if mh == nil {
		mh = h.methodHandlers[""]
	}
	h.mu.RUnlock()

	if mh == nil {
		h.options.PerformError(fmt.Errorf("method %s not allowed", r.Method), r)
		httpError(r, w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	mh.ServeHTTP(w, r)
}

func (h *PatternHandler) Register(method string, in interface{}, do DoFunc, opts ...HandlerOption) *PatternHandler {
	method = strings.ToUpper(method)

	h.mu.Lock()
	defer h.mu.Unlock()

	mh := h.methodHandlers[method]
	if mh != nil {
		panic(fmt.Errorf("method %q already registered", method))
	}
	mh = newMethodhandler(in, do, h.options, opts...)
	h.methodHandlers[method] = mh

	return h
}

type methodHandler struct {
	options *handlerOptions
	in      interface{}
	do      DoFunc
}

func newMethodhandler(in interface{}, do DoFunc, options *handlerOptions, opts ...HandlerOption) (h *methodHandler) {
	h = &methodHandler{
		options: options.Clone(),
		in:      in,
		do:      do,
	}
	newJoinHandlerOption(opts...).apply(h.options)
	return h
}

func (h *methodHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
			h.options.PerformError(fmt.Errorf("unable to send json response: %w", err), r)
			return
		}
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "" {
		err = validateJSONContentType(contentType)
		if err != nil {
			h.options.PerformError(fmt.Errorf("invalid content type %q: %w", contentType, err), r)
			httpError(r, w, "invalid content type", http.StatusBadRequest)
			return
		}
	}

	inVal := reflect.ValueOf(h.in)
	copiedInVal := copyReflectValue(inVal)

	if contentType == "" && (r.Method == http.MethodHead || r.Method == http.MethodGet) {
		if copiedInVal.Elem().Kind() == reflect.Struct {
			err = valuesToStruct(r.URL.Query(), copiedInVal.Interface())
			if err != nil {
				h.options.PerformError(fmt.Errorf("invalid query: %w", err), r)
				httpError(r, w, "invalid query", http.StatusBadRequest)
				return
			}
		}
	} else {
		var rd io.Reader = r.Body
		if h.options.MaxRequestBodySize > 0 {
			rd = io.LimitReader(r.Body, h.options.MaxRequestBodySize)
		}
		var data []byte
		data, err = io.ReadAll(rd)
		if err != nil {
			h.options.PerformError(fmt.Errorf("unable to read request body: %w", err), r)
			httpError(r, w, "unable to read request body", http.StatusBadRequest)
			return
		}
		if len(data) > 0 {
			err = json.Unmarshal(data, copiedInVal.Interface())
			if err != nil {
				h.options.PerformError(fmt.Errorf("unable to unmarshal request body: %w", err), r)
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

	for _, m := range h.options.Middleware {
		m(req, w.Header(), send)
		if sent != 0 {
			return
		}
	}

	h.do(req, w.Header(), send)
}
