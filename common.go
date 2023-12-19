package rapi

import "net/http"

// Request encapsulates http.Request and gives input from request.
// It is used in DoFunc and MiddlewareFunc.
type Request struct {
	*http.Request
	Data []byte
	In   interface{}
}

// Response encapsulates http.Response and gives data and output from response.
// It is returned from Caller.Call.
type Response struct {
	*http.Response
	Data []byte
	Out  interface{}
}

// DoFunc is a function type to process requests from Handler.
type DoFunc func(req *Request, send SendFunc)

// MiddlewareFunc is a function type to process requests as middleware from Handler.
type MiddlewareFunc func(req *Request, send SendFunc, next DoFunc)

// SendFunc is a function type to send response in DoFunc or MiddlewareFunc.
type SendFunc func(out interface{}, code int, header ...http.Header)
