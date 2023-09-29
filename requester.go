package rapi

import (
	"bytes"
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
	header  http.Header
	out     interface{}
	errOut  error
}

func (r *Requester) Do(ctx context.Context, header http.Header, in interface{}) (result *Response, err error) {
	req := (&http.Request{
		Method: r.method,
		URL: &url.URL{
			Scheme:   r.url.Scheme,
			Host:     r.url.Host,
			Path:     r.url.Path,
			RawQuery: "",
		},
		Header: r.header.Clone(),
	}).WithContext(ctx)

	if req.Header == nil {
		req.Header = http.Header{}
	}
	for k, v := range header.Clone() {
		req.Header[k] = v
	}

	var data []byte
	if r.method == http.MethodHead || r.method == http.MethodGet {
		var values url.Values
		values, err = structToValues(in)
		if err != nil {
			return nil, fmt.Errorf("unable to set input to values: %w", err)
		}
		req.URL.RawQuery = values.Encode()
	} else {
		data, err = json.Marshal(in)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal input: %w", err)
		}
		data = append(data, '\n')
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		req.Header.Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
	}
	req.Body = io.NopCloser(bytes.NewBuffer(data))

	resp, err := r.factory.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request error: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	result = &Response{
		Response: resp,
		Out:      nil,
	}

	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		err = validateJSONContentType(contentType)
		if err != nil {
			return result, fmt.Errorf("invalid content type %q: %w", contentType, err)
		}
	}

	var rd io.Reader = resp.Body
	if r.factory.MaxResponseBodySize > 0 {
		rd = io.LimitReader(resp.Body, r.factory.MaxResponseBodySize)
	}
	data, err = io.ReadAll(rd)
	if err != nil {
		return result, fmt.Errorf("unable to read response body: %w", err)
	}

	isErr := resp.StatusCode != http.StatusOK && r.errOut != nil

	outVal := reflect.ValueOf(r.out)
	if isErr {
		outVal = reflect.ValueOf(r.errOut)
	}
	copiedOutVal := copyReflectValue(outVal)

	err = json.Unmarshal(data, copiedOutVal.Interface())
	if err != nil {
		return result, fmt.Errorf("unable to unmarshal response body: %w", err)
	}

	var out interface{}
	if outVal.Kind() == reflect.Pointer {
		out = copiedOutVal.Interface()
	} else {
		out = copiedOutVal.Elem().Interface()
	}

	result.Out = out

	if isErr {
		return result, out.(error)
	}

	return result, nil
}

type Factory struct {
	Client              *http.Client
	URL                 *url.URL
	MaxResponseBodySize int64
}

func (f *Factory) Get(method string, endpoint string, header http.Header, out interface{}, errOut error) *Requester {
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
		header: header.Clone(),
		out:    out,
		errOut: errOut,
	}
}
