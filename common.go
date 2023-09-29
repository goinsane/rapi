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

type DoFunc func(req *Request, respHeader http.Header, send SendFunc)
type SendFunc func(out interface{}, code int)
