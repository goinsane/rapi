package rapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
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
	options          *handlerOptions
	methodHandlersMu sync.RWMutex
	methodHandlers   map[string]*methodHandler
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
	h.methodHandlersMu.RLock()
	mh := h.methodHandlers[r.Method]
	if mh == nil {
		mh = h.methodHandlers[""]
	}
	h.methodHandlersMu.RUnlock()

	if mh == nil {
		h.options.PerformError(fmt.Errorf("method %s not allowed", r.Method), r)
		httpError(r, w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	mh.ServeHTTP(w, r)
}

func (h *PatternHandler) Register(method string, in interface{}, do DoFunc, opts ...HandlerOption) *PatternHandler {
	method = strings.ToUpper(method)

	h.methodHandlersMu.Lock()
	defer h.methodHandlersMu.Unlock()

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
	send := func(out interface{}, header http.Header, code int) {
		var err error

		if !atomic.CompareAndSwapInt32(&sent, 0, 1) {
			panic(errors.New("already sent"))
		}

		for k, v := range header.Clone() {
			for _, v2 := range v {
				w.Header().Add(k, v2)
			}
		}

		if r.Method == http.MethodHead {
			w.WriteHeader(code)
			return
		}

		var data []byte
		if out != nil {
			data, err = json.Marshal(out)
			if err != nil {
				h.options.PerformError(fmt.Errorf("unable to marshal output: %w", err), r)
				httpError(r, w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			data = append(data, '\n')
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
		}

		var wr io.WriteCloser = nopWriteCloser{w}
		if h.options.AllowEncoding {
			wr, err = getContentEncoder(w, r)
			if err != nil {
				h.options.PerformError(fmt.Errorf("unable to get content encoder: %w", err), r)
				return
			}
		}
		defer func(wr io.WriteCloser) {
			_ = wr.Close()
		}(wr)

		w.WriteHeader(code)

		_, err = io.Copy(wr, bytes.NewBuffer(data))
		if err != nil {
			h.options.PerformError(fmt.Errorf("unable to write response body: %w", err), r)
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

	if contentType == "" && copiedInVal.Elem().Kind() == reflect.Struct &&
		(r.Method == http.MethodHead || r.Method == http.MethodGet) {
		err = valuesToStruct(r.URL.Query(), copiedInVal.Interface())
		if err != nil {
			h.options.PerformError(fmt.Errorf("invalid query: %w", err), r)
			httpError(r, w, "invalid query", http.StatusBadRequest)
			return
		}
	} else {
		var rd io.Reader = r.Body
		if h.options.MaxRequestBodySize > 0 {
			rd = io.LimitReader(r.Body, h.options.MaxRequestBodySize)
		}
		completed := make(chan struct{})
		if h.options.RequestTimeout > 0 {
			go func() {
				select {
				case <-time.After(h.options.RequestTimeout):
					_ = r.Body.Close()
				case <-completed:
				}
			}()
		}
		var data []byte
		data, err = io.ReadAll(rd)
		close(completed)
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
		m(req, send)
		if sent != 0 {
			return
		}
	}

	h.do(req, send)
}
