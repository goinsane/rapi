package rapi

import "net/http"

type DoFunc func(req *http.Request, in interface{}, header http.Header, send SendFunc)
type SendFunc func(out interface{}, code int)
