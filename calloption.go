package rapi

import (
	"net/http"
	"net/textproto"
)

// CallOption configures how we set up the http call.
type CallOption interface {
	apply(*callOptions)
}

type funcCallOption struct {
	f func(*callOptions)
}

func (o *funcCallOption) apply(options *callOptions) {
	o.f(options)
}

func newFuncCallOption(f func(options *callOptions)) *funcCallOption {
	return &funcCallOption{
		f: f,
	}
}

type joinCallOption struct {
	opts []CallOption
}

func newJoinCallOption(opts ...CallOption) *joinCallOption {
	return &joinCallOption{
		opts: opts,
	}
}

func (o *joinCallOption) apply(options *callOptions) {
	for _, opt := range o.opts {
		opt.apply(options)
	}
}

type callOptions struct {
	RequestHeader       http.Header
	MaxResponseBodySize int64
	ErrOut              error
	ForceBody           bool
}

func newCallOptions() (o *callOptions) {
	return &callOptions{
		RequestHeader: http.Header{},
	}
}

func (o *callOptions) Clone() *callOptions {
	if o == nil {
		return nil
	}
	result := &callOptions{
		RequestHeader:       o.RequestHeader.Clone(),
		MaxResponseBodySize: o.MaxResponseBodySize,
		ErrOut:              o.ErrOut,
		ForceBody:           o.ForceBody,
	}
	return result
}

// WithRequestHeader returns a CallOption that sets the given http headers to the request headers.
func WithRequestHeader(header ...http.Header) CallOption {
	return newFuncCallOption(func(options *callOptions) {
		for _, hdr := range header {
			for k, v := range hdr.Clone() {
				k = textproto.CanonicalMIMEHeaderKey(k)
				options.RequestHeader[k] = v
			}
		}
	})
}

// WithAdditionalRequestHeader returns a CallOption that adds the given http headers to the request headers.
func WithAdditionalRequestHeader(header ...http.Header) CallOption {
	return newFuncCallOption(func(options *callOptions) {
		for _, hdr := range header {
			for k, v := range hdr {
				for _, v2 := range v {
					options.RequestHeader.Add(k, v2)
				}
			}
		}
	})
}

// WithMaxResponseBodySize returns a CallOption that limits maximum response body size.
func WithMaxResponseBodySize(maxResponseBodySize int64) CallOption {
	return newFuncCallOption(func(options *callOptions) {
		options.MaxResponseBodySize = maxResponseBodySize
	})
}

// WithErrOut returns a CallOption that defines the error output to return as Caller.Call error.
func WithErrOut(errOut error) CallOption {
	return newFuncCallOption(func(options *callOptions) {
		options.ErrOut = errOut
	})
}

// WithForceBody returns a CallOption that forces sending input in the request body for all methods including HEAD and GET.
func WithForceBody(forceBody bool) CallOption {
	return newFuncCallOption(func(options *callOptions) {
		options.ForceBody = forceBody
	})
}
