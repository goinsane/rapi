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
	OnError            func(err error, req *http.Request)
	Middleware         []MiddlewareFunc
	MaxRequestBodySize int64
	RequestTimeout     time.Duration
	ResponseTimeout    time.Duration
	AllowEncoding      bool
}

func newHandlerOptions() (o *handlerOptions) {
	return &handlerOptions{}
}

func (o *handlerOptions) Clone() *handlerOptions {
	if o == nil {
		return nil
	}
	result := &handlerOptions{
		OnError:            o.OnError,
		Middleware:         make([]MiddlewareFunc, len(o.Middleware)),
		MaxRequestBodySize: o.MaxRequestBodySize,
		RequestTimeout:     o.RequestTimeout,
		ResponseTimeout:    o.ResponseTimeout,
		AllowEncoding:      o.AllowEncoding,
	}
	copy(result.Middleware, o.Middleware)
	return result
}

func (o *handlerOptions) PerformError(err error, req *http.Request) {
	if o.OnError != nil {
		o.OnError(err, req)
	}
}

// WithOnError returns a HandlerOption that sets the function to be called on error.
func WithOnError(onError func(error, *http.Request)) HandlerOption {
	return newFuncHandlerOption(func(options *handlerOptions) {
		options.OnError = onError
	})
}

// WithMiddleware returns a HandlerOption that adds middlewares.
func WithMiddleware(middleware ...MiddlewareFunc) HandlerOption {
	return newFuncHandlerOption(func(options *handlerOptions) {
		options.Middleware = append(options.Middleware, middleware...)
	})
}

// WithMaxRequestBodySize returns a HandlerOption that limits maximum request body size.
func WithMaxRequestBodySize(maxRequestBodySize int64) HandlerOption {
	return newFuncHandlerOption(func(options *handlerOptions) {
		options.MaxRequestBodySize = maxRequestBodySize
	})
}

// WithRequestTimeout returns a HandlerOption that limits maximum request duration.
func WithRequestTimeout(requestTimeout time.Duration) HandlerOption {
	return newFuncHandlerOption(func(options *handlerOptions) {
		options.RequestTimeout = requestTimeout
	})
}

// WithResponseTimeout returns a HandlerOption that limits maximum response duration.
func WithResponseTimeout(responseTimeout time.Duration) HandlerOption {
	return newFuncHandlerOption(func(options *handlerOptions) {
		options.ResponseTimeout = responseTimeout
	})
}

// WithAllowEncoding returns a HandlerOption that allows encoded content types such as gzip to be returned.
func WithAllowEncoding(allowEncoding bool) HandlerOption {
	return newFuncHandlerOption(func(options *handlerOptions) {
		options.AllowEncoding = allowEncoding
	})
}
