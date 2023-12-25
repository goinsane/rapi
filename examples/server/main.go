package main

import (
	"log"
	"net/http"
	"time"

	"github.com/goinsane/rapi"
	"github.com/goinsane/rapi/examples/messages"
)

func main() {
	var err error

	handler := rapi.NewHandler(
		rapi.WithOnError(func(err error, req *http.Request) {
			log.Print(err)
		}),
		rapi.WithMaxRequestBodySize(1<<20),
		rapi.WithReadTimeout(60*time.Second),
		rapi.WithWriteTimeout(60*time.Second),
		rapi.WithAllowEncoding(true),
		rapi.WithNotFoundHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
		})),
	)

	handler.Handle("/ping").
		Register(http.MethodGet, new(messages.PingRequest), handlePing)

	handler.Handle("/reverse").
		Register(http.MethodPost, &messages.ReverseRequest{String: "123456789"}, handleReverse,
			rapi.WithMiddleware(func(req *rapi.Request, send rapi.SendFunc, do rapi.DoFunc) {
				in := req.In.(*messages.ReverseRequest)
				if in.String == "" {
					in.String = "filled"
				}
				do(req, send)
			}))

	handler.Handle("/now").
		Register(http.MethodGet, (*messages.NowRequest)(nil), handleNow)

	err = http.ListenAndServe(":8080", handler)
	if err != nil {
		panic(err)
	}
}

func authMiddleware(req *rapi.Request, send rapi.SendFunc, do rapi.DoFunc) {
	if req.Header.Get("X-API-Key") != "1234" {
		send(&messages.ErrorReply{
			ErrorMsg: "unauthorized by api key",
		}, http.StatusUnauthorized)
		return
	}
	do(req, send)
}

func handlePing(req *rapi.Request, send rapi.SendFunc) {
	in := req.In.(*messages.PingRequest)
	out := &messages.PingReply{
		Payload: in.Payload,
	}
	send(out, http.StatusOK)
}

func handleReverse(req *rapi.Request, send rapi.SendFunc) {
	in := req.In.(*messages.ReverseRequest)
	source := []rune(in.String)
	result := make([]rune, 0, len(source))
	for i := len(source) - 1; i >= 0; i-- {
		result = append(result, source[i])
	}
	out := &messages.ReverseReply{
		ReversedString: string(result),
	}
	send(out, http.StatusOK)
}

func handleNow(req *rapi.Request, send rapi.SendFunc) {
	in := req.In.(*messages.NowRequest)
	now := time.Now()
	if in.Time != nil {
		now = *in.Time
	}
	send(&messages.NowReply{
		Now: now.Add(-in.Drift),
	}, http.StatusOK)
}
