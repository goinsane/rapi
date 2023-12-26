package rapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"strconv"
	"strings"
)

// Caller is the HTTP requester to do JSON requests with the given method to the given endpoint.
// The method and endpoint are given from Factory.
type Caller struct {
	options *callOptions
	client  *http.Client
	url     *url.URL
	method  string
	out     interface{}
}

// Call does the HTTP request with the given input and CallOption's.
func (c *Caller) Call(ctx context.Context, in interface{}, opts ...CallOption) (result *Response, err error) {
	options := c.options.Clone()
	newJoinCallOption(opts...).apply(options)

	req := (&http.Request{
		Method: c.method,
		URL: &url.URL{
			Scheme:   c.url.Scheme,
			Host:     c.url.Host,
			Path:     c.url.Path,
			RawQuery: "",
		},
		Header: options.RequestHeader.Clone(),
	}).WithContext(ctx)

	var data []byte
	if inVal := reflect.ValueOf(in); !options.ForceBody &&
		(c.method == http.MethodHead || c.method == http.MethodGet || c.method == http.MethodDelete) {
		if !(in == nil ||
			inVal.Kind() == reflect.Struct || (inVal.Kind() == reflect.Ptr && inVal.Elem().Kind() == reflect.Struct)) {
			return nil, errors.New("input must be nil or struct or struct pointer")
		}
		var values url.Values
		values, err = structToValues(in)
		if err != nil {
			return nil, fmt.Errorf("unable to set input to values: %w", err)
		}
		req.URL.RawQuery = values.Encode()
	} else {
		data, err = json.Marshal(in)
		if err != nil {
			return nil, fmt.Errorf("unable to encode input: %w", err)
		}
		data = append(data, '\n')
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		req.Header.Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
	}
	req.Body = io.NopCloser(bytes.NewBuffer(data))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, &RequestError{err}
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	result = &Response{
		Response: resp,
	}

	var rd io.Reader = resp.Body
	if options.MaxResponseBodySize > 0 {
		rd = io.LimitReader(resp.Body, options.MaxResponseBodySize)
	}

	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		validMediaTypes := []string{"application/json"}
		if resp.StatusCode != http.StatusOK {
			validMediaTypes = append(validMediaTypes, "text/plain")
		}
		var mediaType string
		mediaType, _, err = validateContentType(contentType, validMediaTypes...)
		if err != nil {
			return result, &InvalidContentTypeError{err, contentType}
		}
		if mediaType == "text/plain" {
			data, err = io.ReadAll(io.LimitReader(rd, 1024))
			if err != nil {
				return result, fmt.Errorf("unable to read response body: %w", err)
			}
			return result, &PlainTextError{errors.New(string(data))}
		}
	}

	isErr := resp.StatusCode != http.StatusOK && c.options.ErrOut != nil

	outVal := reflect.ValueOf(c.out)
	if isErr {
		outVal = reflect.ValueOf(c.options.ErrOut)
	}
	copiedOutVal, err := copyReflectValue(outVal)
	if err != nil {
		return result, fmt.Errorf("unable to copy output: %w", err)
	}

	if req.Method != http.MethodHead {
		err = json.NewDecoder(rd).Decode(copiedOutVal.Interface())
		if err != nil {
			return result, fmt.Errorf("unable to decode response body: %w", err)
		}
	}

	var out interface{}
	if outVal.Kind() == reflect.Ptr {
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

// Factory is Caller factory to create new Caller's.
type Factory struct {
	options *callOptions
	client  *http.Client
	url     *url.URL
}

// NewFactory creates a new Factory.
func NewFactory(client *http.Client, u *url.URL, opts ...CallOption) (f *Factory) {
	f = &Factory{
		options: newCallOptions(),
		client:  client,
		url: &url.URL{
			Scheme:   u.Scheme,
			Host:     u.Host,
			Path:     u.Path,
			RawQuery: "",
		},
	}
	newJoinCallOption(opts...).apply(f.options)
	return f
}

// Caller creates a new Caller with the given endpoint and method.
func (f *Factory) Caller(endpoint string, method string, out interface{}, opts ...CallOption) *Caller {
	switch method {
	case http.MethodHead, http.MethodGet, http.MethodDelete:
	case http.MethodPost, http.MethodPut, http.MethodPatch:
	default:
		panic(fmt.Errorf("method %q not allowed", method))
	}

	result := &Caller{
		options: f.options.Clone(),
		client:  f.client,
		url: &url.URL{
			Scheme:   f.url.Scheme,
			Host:     f.url.Host,
			Path:     f.url.Path,
			RawQuery: "",
		},
		method: method,
		out:    out,
	}

	if !strings.HasPrefix(result.url.Path, "/") {
		result.url.Path = "/" + result.url.Path
	}
	if endpoint != "" {
		result.url.Path = path.Join(result.url.Path, endpoint)
	}

	newJoinCallOption(opts...).apply(result.options)
	return result
}
