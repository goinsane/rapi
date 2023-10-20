package main

import (
	"log"
	"net/http"
	"time"

	"github.com/goinsane/rapi"
	"github.com/goinsane/rapi/examples/messages"
)

var startTime = time.Now()

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
				}, http.StatusNotImplemented)
			})

	handler.Handle("/ping").
		Register("GET", new(messages.PingRequest),
			func(req *rapi.Request, send rapi.SendFunc) {
				in := req.In.(*messages.PingRequest)
				send(&messages.PingReply{
					Payload: in.Payload,
				}, http.StatusOK)
			})

	handler.Handle("/echo").
		Register("", nil,
			func(req *rapi.Request, send rapi.SendFunc) {
				send(req.In, http.StatusOK)
			})

	handler.Handle("/test").
		Register("GET", &messages.TestRequest{},
			func(req *rapi.Request, send rapi.SendFunc) {
				in := req.In.(*messages.TestRequest)
				send(&messages.TestReply{
					X:  -in.X,
					T:  in.T,
					B:  in.B,
					BS: string(in.B),
					D:  time.Now().Sub(startTime),
				}, http.StatusOK)
			}, rapi.WithMiddleware(
				func(req *rapi.Request, send rapi.SendFunc, do rapi.DoFunc) {
					in := req.In.(*messages.TestRequest)
					if in.X == 1 {
						send(&messages.TestReply{X: in.X}, http.StatusOK)
					}
					do(req, send)
				},
				func(req *rapi.Request, send rapi.SendFunc, do rapi.DoFunc) {
					in := req.In.(*messages.TestRequest)
					if in.X == 2 {
						send(&messages.TestReply{X: in.X}, http.StatusOK)
					}
					do(req, send)
				},
			))

	err = http.ListenAndServe(":8080", handler)
	if err != nil {
		panic(err)
	}
}
