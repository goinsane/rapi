package rapi

import (
	"net/http"
	"time"
)

type HandlerOption interface {
	apply(*handlerOptions)
}

type funcHandlerOption struct {
	f func(*handlerOptions)
}

func (o *funcHandlerOption) apply(options *handlerOptions) {
	o.f(options)
}

func newFuncHandlerOption(f func(options *handlerOptions)) *funcHandlerOption {
	return &funcHandlerOption{
		f: f,
	}
}

type joinHandlerOption struct {
	opts []HandlerOption
}

func newJoinHandlerOption(opts ...HandlerOption) *joinHandlerOption {
	return &joinHandlerOption{
		opts: opts,
	}
}

func (o *joinHandlerOption) apply(options *handlerOptions) {
	for _, opt := range o.opts {
		opt.apply(options)
	}
}

type handlerOptions struct {
	OnError            func(error, *http.Request)
	Middleware         []DoFunc
	MaxRequestBodySize int64
	RequestTimeout     time.Duration
	AllowEncoding      bool
}

func newHandlerOptions() (o *handlerOptions) {
	return &handlerOptions{
		AllowEncoding: true,
	}
}

func (o *handlerOptions) Clone() *handlerOptions {
	if o == nil {
		return nil
	}
	result := &handlerOptions{
		OnError:            o.OnError,
		Middleware:         make([]DoFunc, len(o.Middleware)),
		MaxRequestBodySize: o.MaxRequestBodySize,
	}
	copy(result.Middleware, o.Middleware)
	return result
}

func (o *handlerOptions) PerformError(err error, req *http.Request) {
	if o.OnError != nil {
		o.OnError(err, req)
	}
}

func WithOnError(onError func(error, *http.Request)) HandlerOption {
	return newFuncHandlerOption(func(options *handlerOptions) {
		options.OnError = onError
	})
}

func WithMiddleware(middleware ...DoFunc) HandlerOption {
	return newFuncHandlerOption(func(options *handlerOptions) {
		options.Middleware = append(options.Middleware, middleware...)
	})
}

func WithMaxRequestBodySize(maxRequestBodySize int64) HandlerOption {
	return newFuncHandlerOption(func(options *handlerOptions) {
		options.MaxRequestBodySize = maxRequestBodySize
	})
}

func WithRequestTimeout(requestTimeout time.Duration) HandlerOption {
	return newFuncHandlerOption(func(options *handlerOptions) {
		options.RequestTimeout = requestTimeout
	})
}

func WithAllowEncoding(allowEncoding bool) HandlerOption {
	return newFuncHandlerOption(func(options *handlerOptions) {
		options.AllowEncoding = allowEncoding
	})
}
