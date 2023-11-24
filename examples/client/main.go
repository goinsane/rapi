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
