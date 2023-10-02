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

	unimplemented := func(req *rapi.Request, header http.Header, send rapi.SendFunc) {
		send(&messages.ErrorResponse{
			ErrorMsg: http.StatusText(http.StatusNotImplemented),
		}, http.StatusNotImplemented)
	}

	handler.Handle("/").
		Register("", nil, unimplemented).
		Register("HEAD", struct{}{}, unimplemented).
		Register("GET", struct{}{}, unimplemented)

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
