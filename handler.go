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

	"github.com/goinsane/logng"
)

type Handler struct {
	Logger             *logng.Logger
	Middleware         []DoFunc
	MaxRequestBodySize int64

	handlersMu sync.RWMutex
	handlers   map[string]*_PureHandler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := h.Logger.WithPrefixf("%s %s: ", r.Method, r.RequestURI)

	h.handlersMu.RLock()
	ph := h.handlers[r.Method]
	if ph == nil {
		ph = h.handlers[""]
	}
	h.handlersMu.RUnlock()
	if ph == nil {
		sendJSONResponse(logger, w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
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

type _PureHandler struct {
	Handler    *Handler
	In         interface{}
	Do         DoFunc
	Middleware []DoFunc
}

func (h *_PureHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger := h.Handler.Logger.WithPrefixf("%s %s: ", req.Method, req.RequestURI)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(req.Body)

	var sent int32
	send := func(out interface{}, code int) {
		if !atomic.CompareAndSwapInt32(&sent, 0, 1) {
			panic(errors.New("already sent"))
		}
		sendJSONResponse(logger, w, out, code)
	}

	var rd io.Reader = req.Body
	if h.Handler.MaxRequestBodySize > 0 {
		rd = io.LimitReader(req.Body, h.Handler.MaxRequestBodySize)
	}
	data, err := io.ReadAll(rd)
	if err != nil {
		logger.Errorf("unable to read request body: %w", err)
		return
	}

	inVal := reflect.ValueOf(h.In)
	copiedInVal := copyReflectValue(inVal)

	err = json.Unmarshal(data, copiedInVal.Interface())
	if err != nil {
		logger.Errorf("unable to unmarshal request body: %w", err)
		send("unable to unmarshal request body", http.StatusBadRequest)
		return
	}

	var in interface{}
	if inVal.Kind() == reflect.Pointer {
		in = copiedInVal.Interface()
	} else {
		in = copiedInVal.Elem().Interface()
	}

	for _, m := range h.Handler.Middleware {
		m(req, in, w.Header(), send)
		if sent != 0 {
			return
		}
	}

	for _, m := range h.Middleware {
		m(req, in, w.Header(), send)
		if sent != 0 {
			return
		}
	}

	h.Do(req, in, w.Header(), send)
}
