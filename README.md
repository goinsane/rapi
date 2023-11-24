# rAPI

[![Go Reference](https://pkg.go.dev/badge/github.com/goinsane/rapi.svg)](https://pkg.go.dev/github.com/goinsane/rapi)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=goinsane_rapi&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=goinsane_rapi)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=goinsane_rapi&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=goinsane_rapi)

**rAPI** is a Go (Golang) package that simplifies building and consuming RESTful APIs. It provides an HTTP handler for
creating APIs and a client for making API requests.

## Features

### Handler features

- Handling by pattern and method
- Accepting query string or request body on GET and HEAD methods
- Setting various options by using HandlerOption's
- Middleware support as a HandleOption

### Caller features

- Calling by endpoint and method
- Ability to force request body in GET and HEAD methods
- Setting various options by using CallOption's

## Installation

You can install **rAPI** using the `go get` command:

```sh
go get github.com/goinsane/rapi
```

## Examples

### Server example

```go
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
	)

	handler.Handle("/").
		Register("", nil, handleUnimplemented)

	handler.Handle("/echo", rapi.WithMiddleware(authMiddleware)).
		Register("", nil, handleEcho)

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

func handleUnimplemented(req *rapi.Request, send rapi.SendFunc) {
	send(&messages.ErrorReply{
		ErrorMsg: http.StatusText(http.StatusNotImplemented),
	}, http.StatusNotImplemented)
}

func handleEcho(req *rapi.Request, send rapi.SendFunc) {
	hdr := http.Header{}
	hdr.Set("X-Request-Method", req.Method)
	send(req.In, http.StatusOK, hdr)
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

```

### Client example

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/goinsane/rapi"
	"github.com/goinsane/rapi/examples/messages"
)

func main() {
	u, _ := url.Parse("http://127.0.0.1:8080")
	factory := rapi.NewFactory(http.DefaultClient, u, rapi.WithErrOut(new(messages.ErrorReply)))

	resp, err := factory.Caller("/reverse", http.MethodGet, &messages.ReverseReply{}).
		Call(context.TODO(), &messages.ReverseRequest{
			String: "abcdefgh",
		})
	if err != nil {
		out := err.(*messages.ErrorReply)
		panic(out)
	}
	out := resp.Out.(*messages.ReverseReply)
	fmt.Println(out)
}

```

### More examples

To run any example, please use the command like the following:

```sh
cd examples/server/
go run *.go
```

## Contributing

We welcome contributions from the community to improve and expand project capabilities. If you find a bug, have a
feature request, or want to contribute code, please follow our guidelines for contributing
([CONTRIBUTING.md](CONTRIBUTING.md)) and submit a pull request.

## License

This project is licensed under the [BSD 3-Clause License](LICENSE).
