package rapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
)

type Requester struct {
	Client              *http.Client
	Method              string
	URL                 *url.URL
	Out                 interface{}
	MaxResponseBodySize int64
}

func (r *Requester) Do(ctx context.Context, header http.Header, in interface{}) (out interface{}, resp *http.Response, err error) {
	req := (&http.Request{
		Method: r.Method,
		URL:    r.URL,
		Header: header.Clone(),
	}).WithContext(ctx)

	data, err := json.Marshal(in)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal input: %w", err)
	}
	data = append(data, '\n')
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))

	resp, err = r.Client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("http request error: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	outVal := reflect.ValueOf(r.Out)
	copiedOutVal := copyReflectValue(outVal)

	var rd io.Reader = req.Body
	if r.MaxResponseBodySize > 0 {
		rd = io.LimitReader(req.Body, r.MaxResponseBodySize)
	}
	data, err = io.ReadAll(rd)
	if err != nil {
		return nil, resp, fmt.Errorf("unable to read response body: %w", err)
	}

	err = json.Unmarshal(data, copiedOutVal.Interface())
	if err != nil {
		return nil, resp, fmt.Errorf("unable to unmarshal response body: %w", err)
	}

	if outVal.Kind() == reflect.Pointer {
		out = copiedOutVal.Interface()
	} else {
		out = copiedOutVal.Elem().Interface()
	}

	return out, resp, nil
}
