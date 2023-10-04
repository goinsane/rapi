package rapi

import "net/http"

type Request struct {
	*http.Request
	In        interface{}
	Transport interface{}
}

type Response struct {
	*http.Response
	Data []byte
	Out  interface{}
}

type DoFunc func(req *Request, send SendFunc)
type SendFunc func(out interface{}, header http.Header, code int)
