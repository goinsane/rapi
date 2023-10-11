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
	ErrOut              error
	ForceBody           bool
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
		ErrOut:              o.ErrOut,
		ForceBody:           o.ForceBody,
	}
	return result
}

func WithRequestHeader(header ...http.Header) CallerOption {
	return newFuncCallerOption(func(options *callerOptions) {
		for _, hdr := range header {
			for k, v := range hdr.Clone() {
				k = textproto.CanonicalMIMEHeaderKey(k)
				options.RequestHeader[k] = v
			}
		}
	})
}

func WithAdditionalRequestHeader(header ...http.Header) CallerOption {
	return newFuncCallerOption(func(options *callerOptions) {
		for _, hdr := range header {
			for k, v := range hdr {
				for _, v2 := range v {
					options.RequestHeader.Add(k, v2)
				}
			}
		}
	})
}

func WithMaxResponseBodySize(maxResponseBodySize int64) CallerOption {
	return newFuncCallerOption(func(options *callerOptions) {
		options.MaxResponseBodySize = maxResponseBodySize
	})
}

func WithErrOut(errOut error) CallerOption {
	return newFuncCallerOption(func(options *callerOptions) {
		options.ErrOut = errOut
	})
}

func WithForceBody(forceBody bool) CallerOption {
	return newFuncCallerOption(func(options *callerOptions) {
		options.ForceBody = forceBody
	})
}
