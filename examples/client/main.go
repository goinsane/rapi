package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/goinsane/rapi"
)

func main() {
	u, _ := url.Parse("http://127.0.0.1:8080")
	f := rapi.NewFactory(http.DefaultClient, u)
	c := f.Caller("/echo", http.MethodGet, nil)
	resp, err := c.Call(context.TODO(), nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(resp.Out, resp.Header)
}
