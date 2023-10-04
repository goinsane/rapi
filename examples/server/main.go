package main

import (
	"log"
	"net/http"

	"github.com/goinsane/rapi"
	"github.com/goinsane/rapi/examples/messages"
)

func main() {
	var err error

	handler := rapi.NewHandler(rapi.WithOnError(func(err error, req *http.Request) {
		log.Print(err)
	}))

	handler.Handle("/").
		Register("", nil,
			func(req *rapi.Request, send rapi.SendFunc) {
				send(&messages.ErrorResponse{
					ErrorMsg: http.StatusText(http.StatusNotImplemented),
				}, nil, http.StatusNotImplemented)
			})

	handler.Handle("/ping").
		Register("GET", new(messages.PingRequest),
			func(req *rapi.Request, send rapi.SendFunc) {
				in := req.In.(*messages.PingRequest)
				send(&messages.PingReply{
					Payload: in.Payload,
				}, nil, http.StatusOK)
			})

	handler.Handle("/echo").
		Register("POST", nil,
			func(req *rapi.Request, send rapi.SendFunc) {
				send(req.In, nil, http.StatusOK)
			})

	handler.Handle("/test").
		Register("GET", &messages.TestRequest{},
			func(req *rapi.Request, send rapi.SendFunc) {
				in := req.In.(*messages.TestRequest)
				send(&messages.TestReply{X: -in.X}, nil, http.StatusOK)
			}, rapi.WithMiddleware(
				func(req *rapi.Request, send rapi.SendFunc, do rapi.DoFunc) {
					in := req.In.(*messages.TestRequest)
					if in.X == 1 {
						send(&messages.TestReply{X: in.X}, nil, http.StatusOK)
					}
					do(req, send)
				},
				func(req *rapi.Request, send rapi.SendFunc, do rapi.DoFunc) {
					in := req.In.(*messages.TestRequest)
					if in.X == 2 {
						send(&messages.TestReply{X: in.X}, nil, http.StatusOK)
					}
					do(req, send)
				},
			))

	err = http.ListenAndServe(":8080", handler)
	if err != nil {
		panic(err)
	}
}
