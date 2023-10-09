package rapi

import "net/http"

type Request struct {
	*http.Request
	In interface{}
}

type Response struct {
	*http.Response
	Data []byte
	Out  interface{}
}

type DoFunc func(req *Request, send SendFunc)
type MiddlewareFunc func(req *Request, send SendFunc, do DoFunc)
type SendFunc func(out interface{}, code int, header ...http.Header)
