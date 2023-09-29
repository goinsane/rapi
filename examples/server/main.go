package main

import (
	"log"
	"net/http"

	"github.com/goinsane/rapi"
	"github.com/goinsane/rapi/examples/messages"
)

func main() {
	var err error

	handler := &rapi.Handler{
		OnError: func(err error, request *http.Request) {
			log.Print(err)
		},
	}

	handler.Handle("/").Register("", new(interface{}),
		func(req *rapi.Request, header http.Header, send rapi.SendFunc) {
			send(&messages.ErrorResponse{
				ErrorMsg: http.StatusText(http.StatusNotImplemented),
			}, http.StatusNotImplemented)
		})

	handler.Handle("/ping").Register("GET", new(messages.PingRequest),
		func(req *rapi.Request, respHeader http.Header, send rapi.SendFunc) {
			in := req.In.(*messages.PingRequest)
			send(&messages.PingReply{
				Payload: in.Payload,
			}, http.StatusOK)
		})

	err = http.ListenAndServe(":8080", handler)
	if err != nil {
		panic(err)
	}
}
