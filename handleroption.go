package rapi

import (
	"net/http"
	"time"
)

// HandlerOption sets options such as middleware, read timeout, etc.
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
	Middlewares        []MiddlewareFunc
	MaxRequestBodySize int64
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	AllowEncoding      bool
	OptionsHandler     http.Handler
	NotFoundHandler    http.Handler
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
		Middlewares:        make([]MiddlewareFunc, len(o.Middlewares)),
		MaxRequestBodySize: o.MaxRequestBodySize,
		ReadTimeout:        o.ReadTimeout,
		WriteTimeout:       o.WriteTimeout,
		AllowEncoding:      o.AllowEncoding,
		OptionsHandler:     o.OptionsHandler,
		NotFoundHandler:    o.NotFoundHandler,
	}
	copy(result.Middlewares, o.Middlewares)
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
func WithMiddleware(middlewares ...MiddlewareFunc) HandlerOption {
	return newFuncHandlerOption(func(options *handlerOptions) {
		options.Middlewares = append(options.Middlewares, middlewares...)
	})
}

// WithMaxRequestBodySize returns a HandlerOption that limits maximum request body size.
func WithMaxRequestBodySize(maxRequestBodySize int64) HandlerOption {
	return newFuncHandlerOption(func(options *handlerOptions) {
		options.MaxRequestBodySize = maxRequestBodySize
	})
}

// WithReadTimeout returns a HandlerOption that limits maximum request body read duration.
func WithReadTimeout(readTimeout time.Duration) HandlerOption {
	return newFuncHandlerOption(func(options *handlerOptions) {
		options.ReadTimeout = readTimeout
	})
}

// WithWriteTimeout returns a HandlerOption that limits maximum response body write duration.
func WithWriteTimeout(writeTimeout time.Duration) HandlerOption {
	return newFuncHandlerOption(func(options *handlerOptions) {
		options.WriteTimeout = writeTimeout
	})
}

// WithAllowEncoding returns a HandlerOption that allows encoded content types such as gzip to be returned.
// By default, encoding is allowed.
func WithAllowEncoding(allowEncoding bool) HandlerOption {
	return newFuncHandlerOption(func(options *handlerOptions) {
		options.AllowEncoding = allowEncoding
	})
}

// WithOptionsHandler returns a HandlerOption that handles requests with method OPTIONS.
func WithOptionsHandler(optionsHandler http.Handler) HandlerOption {
	return newFuncHandlerOption(func(options *handlerOptions) {
		options.OptionsHandler = optionsHandler
	})
}

// WithNotFoundHandler returns a HandlerOption that handles requests when the pattern isn't match.
func WithNotFoundHandler(notFoundHandler http.Handler) HandlerOption {
	return newFuncHandlerOption(func(options *handlerOptions) {
		options.NotFoundHandler = notFoundHandler
	})
}
