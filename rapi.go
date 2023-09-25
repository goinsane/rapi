package rapi

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/goinsane/logng"
)

type DoFunc func(req *http.Request, in interface{}, send SendFunc)
type SendFunc func(out interface{}, header http.Header, code int)

type Handler struct {
	Logger             *logng.Logger
	MaxRequestBodySize int64

	handlersMu sync.RWMutex
	handlers   map[string]*_PureHandler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := h.Logger.WithPrefixf("%s: ", r.Method)

	h.handlersMu.RLock()
	ph := h.handlers[r.Method]
	if ph == nil {
		ph = h.handlers[""]
	}
	h.handlersMu.RUnlock()
	if ph == nil {
		sendResponse(logger, w, http.StatusText(http.StatusMethodNotAllowed), nil, http.StatusMethodNotAllowed)
		return
	}

	ph.ServeHTTP(w, r)
}

func (h *Handler) Register(method string, in interface{}, do DoFunc) *Handler {
	method = strings.ToUpper(method)
	h.handlersMu.Lock()
	defer h.handlersMu.Unlock()
	if h.handlers == nil {
		h.handlers = make(map[string]*_PureHandler)
	}
	ph := h.handlers[method]
	if ph != nil {
		panic("method already registered")
	}
	ph = &_PureHandler{
		Handler: h,
		In:      in,
		Do:      do,
	}
	h.handlers[method] = ph
	return h
}

type _PureHandler struct {
	Handler *Handler
	In      interface{}
	Do      DoFunc
}

func (h *_PureHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := h.Handler.Logger.WithPrefixf("%s: ", r.Method)

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
		sendResponse(logger, w, out, header, code)
	}

	var rd io.Reader = r.Body
	if h.Handler.MaxRequestBodySize > 0 {
		rd = io.LimitReader(r.Body, h.Handler.MaxRequestBodySize)
	}
	data, err := io.ReadAll(rd)
	if err != nil {
		logger.Errorf("unable to read request body: %w", err)
		return
	}

	err = json.Unmarshal(data, copiedInVal.Interface())
	if err != nil {
		logger.Errorf("unable to unmarshal request body from json: %w", err)
		send("unable to unmarshal request body from json", nil, http.StatusBadRequest)
		return
	}

	var in interface{}
	if !isInValPtr {
		in = copiedInVal.Elem().Interface()
	} else {
		in = copiedInVal.Interface()
	}

	h.Do(r, in, send)
}
