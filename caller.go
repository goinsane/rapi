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

type Caller struct {
	options *callerOptions
	client  *http.Client
	url     *url.URL
	method  string
	out     interface{}
	errOut  error
}

func (c *Caller) Call(ctx context.Context, in interface{}, opts ...CallerOption) (result *Response, err error) {
	options := c.options.Clone()
	newJoinCallerOption(opts...).apply(options)

	req := (&http.Request{
		Method: c.method,
		URL: &url.URL{
			Scheme:   c.url.Scheme,
			Host:     c.url.Host,
			Path:     c.url.Path,
			RawQuery: "",
		},
		Header: options.RequestHeader,
	}).WithContext(ctx)

	var data []byte
	if in != nil {
		if inValType := reflect.ValueOf(in).Type(); (inValType.Kind() == reflect.Struct ||
			(inValType.Kind() == reflect.Pointer &&
				inValType.Elem().Kind() == reflect.Struct)) &&
			(c.method == http.MethodHead || c.method == http.MethodGet) {
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
	}
	req.Body = io.NopCloser(bytes.NewBuffer(data))

	resp, err := c.client.Do(req)
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
	if options.MaxResponseBodySize > 0 {
		rd = io.LimitReader(resp.Body, options.MaxResponseBodySize)
	}
	data, err = io.ReadAll(rd)
	if err != nil {
		return result, fmt.Errorf("unable to read response body: %w", err)
	}
	result.Data = data

	isErr := resp.StatusCode != http.StatusOK && c.errOut != nil

	outVal := reflect.ValueOf(c.out)
	if isErr {
		outVal = reflect.ValueOf(c.errOut)
	}
	copiedOutVal := copyReflectValue(outVal)

	if len(data) > 0 || isErr {
		err = json.Unmarshal(data, copiedOutVal.Interface())
		if err != nil {
			return result, fmt.Errorf("unable to unmarshal response body: %w", err)
		}
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
	options *callerOptions
	client  *http.Client
	url     *url.URL
}

func NewFactory(client *http.Client, u *url.URL) (f *Factory) {
	f = &Factory{
		options: newCallerOptions(),
		client:  client,
		url: &url.URL{
			Scheme:   u.Scheme,
			Host:     u.Host,
			Path:     u.Path,
			RawQuery: "",
		},
	}
	return f
}

func (f *Factory) Caller(endpoint string, method string, out interface{}, errOut error, opts ...CallerOption) *Caller {
	result := &Caller{
		options: f.options.Clone(),
		client:  f.client,
		url: &url.URL{
			Scheme:   f.url.Scheme,
			Host:     f.url.Host,
			Path:     path.Join(f.url.Path, endpoint),
			RawQuery: "",
		},
		method: method,
		out:    out,
		errOut: errOut,
	}
	newJoinCallerOption(opts...).apply(f.options)
	return result
}
