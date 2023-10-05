package rapi

import (
	"net/http"
	"net/textproto"
)

type CallerOption interface {
	apply(*callerOptions)
}

type funcCallerOption struct {
	f func(*callerOptions)
}

func (o *funcCallerOption) apply(options *callerOptions) {
	o.f(options)
}

func newFuncCallerOption(f func(options *callerOptions)) *funcCallerOption {
	return &funcCallerOption{
		f: f,
	}
}

type joinCallerOption struct {
	opts []CallerOption
}

func newJoinCallerOption(opts ...CallerOption) *joinCallerOption {
	return &joinCallerOption{
		opts: opts,
	}
}

func (o *joinCallerOption) apply(options *callerOptions) {
	for _, opt := range o.opts {
		opt.apply(options)
	}
}

type callerOptions struct {
	RequestHeader       http.Header
	MaxResponseBodySize int64
}

func newCallerOptions() (o *callerOptions) {
	return &callerOptions{
		RequestHeader: http.Header{},
	}
}

func (o *callerOptions) Clone() *callerOptions {
	if o == nil {
		return nil
	}
	result := &callerOptions{
		RequestHeader:       o.RequestHeader.Clone(),
		MaxResponseBodySize: o.MaxResponseBodySize,
	}
	return result
}

func WithRequestHeader(requestHeader http.Header) CallerOption {
	return newFuncCallerOption(func(options *callerOptions) {
		for k, v := range requestHeader.Clone() {
			k = textproto.CanonicalMIMEHeaderKey(k)
			options.RequestHeader[k] = v
		}
	})
}

func WithAdditionalRequestHeader(requestHeader http.Header) CallerOption {
	return newFuncCallerOption(func(options *callerOptions) {
		for k, v := range requestHeader {
			k = textproto.CanonicalMIMEHeaderKey(k)
			for _, v2 := range v {
				options.RequestHeader.Add(k, v2)
			}
		}
	})
}

func WithMaxResponseBodySize(maxResponseBodySize int64) CallerOption {
	return newFuncCallerOption(func(options *callerOptions) {
		options.MaxResponseBodySize = maxResponseBodySize
	})
}
