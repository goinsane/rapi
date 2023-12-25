package rapi

import "fmt"

// InvalidContentTypeError occurs when the request or response body content type is invalid.
type InvalidContentTypeError struct {
	error       error
	contentType string
}

// Error is the implementation of error.
func (e *InvalidContentTypeError) Error() string {
	return fmt.Errorf("invalid content type %q: %w", e.contentType, e.error).Error()
}

// Unwrap unwraps the underlying error.
func (e *InvalidContentTypeError) Unwrap() error {
	return e.error
}

// ContentType returns the invalid content type.
func (e *InvalidContentTypeError) ContentType() string {
	return e.contentType
}

// RequestError is the request error from http.Client.
// It is returned from Caller.Call.
type RequestError struct{ error error }

// Error is the implementation of error.
func (e *RequestError) Error() string {
	return fmt.Errorf("request error: %w", e.error).Error()
}

// Unwrap unwraps the underlying error.
func (e *RequestError) Unwrap() error {
	return e.error
}

// PlainTextError is the plain text error returned from http server.
// It is returned from Caller.Call.
type PlainTextError struct{ error error }

// Error is the implementation of error.
func (e *PlainTextError) Error() string {
	return fmt.Errorf("plain text error: %w", e.error).Error()
}

// Unwrap unwraps the underlying error.
func (e *PlainTextError) Unwrap() error {
	return e.error
}
