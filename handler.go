package rapi

import (
	"bytes"
	"context"
	"encoding/json"
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

// Handler implements http.Handler to process JSON requests based on pattern and registered methods.
// Handler is similar to http.ServeMux in terms of operation.
type Handler struct {
	options  *handlerOptions
	serveMux *http.ServeMux
}

// NewHandler creates a new Handler by given HandlerOption's.
func NewHandler(opts ...HandlerOption) (h *Handler) {
	h = &Handler{
		options:  newHandlerOptions(),
		serveMux: http.NewServeMux(),
	}
	newJoinHandlerOption(opts...).apply(h.options)
	return h
}

// ServeHTTP is the implementation of http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.serveMux.ServeHTTP(w, r)
}

// Handle creates a Registrar to register methods for the given pattern.
func (h *Handler) Handle(pattern string, opts ...HandlerOption) Registrar {
	ph := newPatternHandler(h.options, opts...)
	h.serveMux.Handle(pattern, ph)
	return &struct{ Registrar }{ph}
}

// Registrar is method registrar and created by Handler.Handle.
type Registrar interface {
	// Register registers method with the given parameters to Handler. The pattern was given from Handler.Handle.
	Register(method string, in interface{}, do DoFunc, opts ...HandlerOption) Registrar
}

type patternHandler struct {
	options          *handlerOptions
	methodHandlersMu sync.RWMutex
	methodHandlers   map[string]*methodHandler
}

func newPatternHandler(options *handlerOptions, opts ...HandlerOption) (h *patternHandler) {
	h = &patternHandler{
		options:        options.Clone(),
		methodHandlers: make(map[string]*methodHandler),
	}
	newJoinHandlerOption(opts...).apply(h.options)
	return h
}

func (h *patternHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (h *patternHandler) Register(method string, in interface{}, do DoFunc, opts ...HandlerOption) Registrar {
	method = strings.ToUpper(method)

	switch method {
	case http.MethodGet:
	case http.MethodPost:
	case http.MethodPut:
	case http.MethodPatch:
	case http.MethodDelete:
	default:
		panic(fmt.Errorf("method %q not allowed", method))
	}

	h.methodHandlersMu.Lock()
	defer h.methodHandlersMu.Unlock()

	mh := h.methodHandlers[method]
	if mh != nil {
		panic(fmt.Errorf("method %q already registered", method))
	}
	mh = newMethodhandler(in, do, h.options, opts...)
	h.methodHandlers[method] = mh
	if method == http.MethodGet {
		h.methodHandlers[http.MethodHead] = mh
	}

	return &struct{ Registrar }{h}
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

	req := &Request{
		Request: r,
	}

	var sent int32
	send := func(out interface{}, code int, header ...http.Header) {
		var err error

		if !atomic.CompareAndSwapInt32(&sent, 0, 1) {
			return
		}

		var nopwc io.WriteCloser = nopWriteCloser{w}
		wc := nopwc
		if h.options.AllowEncoding {
			wc, err = getContentEncoder(w, r.Header.Get("Accept-Encoding"))
			if err != nil {
				h.options.PerformError(fmt.Errorf("unable to get content encoder: %w", err), r)
				httpError(r, w, "invalid accept encoding", http.StatusBadRequest)
				return
			}
		}

		var data []byte
		data, err = json.Marshal(out)
		if err != nil {
			h.options.PerformError(fmt.Errorf("unable to marshal output: %w", err), r)
			httpError(r, w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		data = append(data, '\n')

		for _, hdr := range header {
			for k, v := range hdr {
				for _, v2 := range v {
					w.Header().Add(k, v2)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if wc == nopwc {
			w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
		} else {
			w.Header().Del("Content-Length")
		}
		w.WriteHeader(code)
		if r.Method == http.MethodHead {
			return
		}

		respCtx, respCancel := context.WithCancel(context.Background())
		defer respCancel()

		if h.options.WriteTimeout > 0 {
			go func() {
				select {
				case <-time.After(h.options.WriteTimeout):
					respCancel()
				case <-respCtx.Done():
				}
			}()
		}

		go func() {
			defer respCancel()

			var err error

			_, err = io.Copy(wc, bytes.NewBuffer(data))
			if err != nil {
				h.options.PerformError(fmt.Errorf("unable to write response body: %w", err), r)
				return
			}

			err = wc.Close()
			if err != nil {
				h.options.PerformError(fmt.Errorf("unable to write end of response body: %w", err), r)
				return
			}
		}()

		<-respCtx.Done()
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "" {
		_, _, err = validateContentType(contentType, "application/json")
		if err != nil {
			h.options.PerformError(&InvalidContentTypeError{err, contentType}, r)
			httpError(r, w, "invalid content type", http.StatusBadRequest)
			return
		}
	}

	inVal := reflect.ValueOf(h.in)
	copiedInVal, err := copyReflectValue(inVal)
	if err != nil {
		h.options.PerformError(fmt.Errorf("unable to copy input: %w", err), r)
		httpError(r, w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

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
		if h.options.ReadTimeout > 0 {
			go func() {
				select {
				case <-time.After(h.options.ReadTimeout):
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
		req.Data = data
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
	if inVal.Kind() == reflect.Ptr {
		in = copiedInVal.Interface()
	} else {
		in = copiedInVal.Elem().Interface()
	}

	req.In = in

	do := []DoFunc{
		func(req *Request, send SendFunc) {
			if sent == 0 && h.do != nil {
				h.do(req, send)
			}
		},
	}
	for i := len(h.options.Middleware) - 1; i >= 0; i-- {
		m := h.options.Middleware[i]
		l := len(do)
		do = append(do, func(req *Request, send SendFunc) {
			if sent == 0 && m != nil {
				m(req, send, do[l-1])
			}
		})
	}
	do[len(do)-1](req, send)
}
