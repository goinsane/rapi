package rapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"strconv"
)

type Requester struct {
	factory *Factory
	method  string
	url     *url.URL
	out     interface{}
}

func (r *Requester) Do(ctx context.Context, header http.Header, in interface{}) (out interface{}, resp *http.Response, err error) {
	req := (&http.Request{
		Method: r.method,
		URL:    r.url,
		Header: header.Clone(),
	}).WithContext(ctx)

	data, err := json.Marshal(in)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal input: %w", err)
	}
	data = append(data, '\n')
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))

	resp, err = r.factory.Client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("http request error: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	var rd io.Reader = resp.Body
	if r.factory.MaxResponseBodySize > 0 {
		rd = io.LimitReader(resp.Body, r.factory.MaxResponseBodySize)
	}
	data, err = io.ReadAll(rd)
	if err != nil {
		return nil, resp, fmt.Errorf("unable to read response body: %w", err)
	}

	outVal := reflect.ValueOf(r.out)
	copiedOutVal := copyReflectValue(outVal)

	err = json.Unmarshal(data, copiedOutVal.Interface())
	if err != nil {
		return data, resp, fmt.Errorf("unable to unmarshal response body: %w", err)
	}

	if outVal.Kind() == reflect.Pointer {
		out = copiedOutVal.Interface()
	} else {
		out = copiedOutVal.Elem().Interface()
	}

	return out, resp, nil
}

type Factory struct {
	Client              *http.Client
	URL                 *url.URL
	MaxResponseBodySize int64
}

func (f *Factory) Get(method string, endpoint string, out interface{}) *Requester {
	if out == nil {
		panic("output is nil")
	}
	return &Requester{
		factory: f,
		method:  method,
		url: &url.URL{
			Scheme:   f.URL.Scheme,
			Host:     f.URL.Host,
			Path:     path.Join(f.URL.Path, endpoint),
			RawQuery: "",
		},
		out: out,
	}
}
