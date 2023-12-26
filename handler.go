package rapi

import (
	"bytes"
	"context"
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
	switch r.Method {
	case http.MethodHead:
	case http.MethodGet:
	case http.MethodPost:
	case http.MethodPut:
	case http.MethodPatch:
	case http.MethodDelete:
	case http.MethodOptions:
		if h.options.OptionsHandler == nil {
			h.options.PerformError(fmt.Errorf("method %s handler not defined", r.Method), r)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		h.options.OptionsHandler.ServeHTTP(w, r)
		return
	default:
		h.options.PerformError(fmt.Errorf("method %s not allowed", r.Method), r)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if r.RequestURI == "*" {
		if r.ProtoAtLeast(1, 1) {
			w.Header().Set("Connection", "close")
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	handler, pattern := h.serveMux.Handler(r)
	if h.options.NotFoundHandler != nil && pattern == "" {
		h.options.NotFoundHandler.ServeHTTP(w, r)
		return
	}

	handler.ServeHTTP(w, r)
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
		h.options.PerformError(fmt.Errorf("method %s not registered", r.Method), r)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	mh.ServeHTTP(w, r)
}

func (h *patternHandler) Register(method string, in interface{}, do DoFunc, opts ...HandlerOption) Registrar {
	inVal, err := copyReflectValue(reflect.ValueOf(in))
	if err != nil {
		panic(fmt.Errorf("unable to copy input: %w", err))
	}

	method = strings.ToUpper(method)

	switch method {
	case "", http.MethodGet, http.MethodDelete:
		if inVal.Elem().Kind() != reflect.Struct {
			panic(errors.New("input must be struct or struct pointer"))
		}
	case http.MethodPost, http.MethodPut, http.MethodPatch:
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
	send := func(out interface{}, code int, headers ...http.Header) {
		var err error

		if !atomic.CompareAndSwapInt32(&sent, 0, 1) {
			panic(errors.New("already sent"))
		}

		var nopcw io.WriteCloser = nopCloserForWriter{w}
		wc := nopcw
		if h.options.AllowEncoding {
			wc, err = getContentEncoder(w, r.Header.Get("Accept-Encoding"))
			if err != nil {
				h.options.PerformError(fmt.Errorf("unable to get content encoder: %w", err), r)
				http.Error(w, "invalid accept encoding", http.StatusBadRequest)
				return
			}
		}

		var data []byte
		data, err = json.Marshal(out)
		if err != nil {
			panic(fmt.Errorf("unable to encode output: %w", err))
		}
		data = append(data, '\n')

		for _, hdr := range headers {
			for k, v := range hdr {
				for _, v2 := range v {
					w.Header().Add(k, v2)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if wc == nopcw {
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
			http.Error(w, "invalid content type", http.StatusBadRequest)
			return
		}
	}

	inVal := reflect.ValueOf(h.in)
	copiedInVal, err := copyReflectValue(inVal)
	if err != nil {
		panic(fmt.Errorf("unable to copy input: %w", err))
	}

	if contentType == "" &&
		(r.Method == http.MethodHead || r.Method == http.MethodGet || r.Method == http.MethodDelete) {
		if copiedInVal.Elem().Kind() != reflect.Struct {
			panic(errors.New("input must be struct or struct pointer"))
		}
		err = valuesToStruct(r.URL.Query(), copiedInVal.Interface())
		if err != nil {
			h.options.PerformError(fmt.Errorf("invalid query: %w", err), r)
			http.Error(w, "invalid query", http.StatusBadRequest)
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
		err = json.NewDecoder(rd).Decode(copiedInVal.Interface())
		close(completed)
		if err != nil {
			h.options.PerformError(fmt.Errorf("unable to decode request body: %w", err), r)
			http.Error(w, "unable to decode request body", http.StatusBadRequest)
			return
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
	for i := len(h.options.Middlewares) - 1; i >= 0; i-- {
		m := h.options.Middlewares[i]
		l := len(do)
		do = append(do, func(req *Request, send SendFunc) {
			if sent == 0 && m != nil {
				m(req, send, do[l-1])
			}
		})
	}
	do[len(do)-1](req, send)

	if sent == 0 {
		panic(errors.New("send must be called"))
	}
}
