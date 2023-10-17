package rapi

import "fmt"

type RequestError struct{ error }

func (e *RequestError) Error() string {
	return fmt.Errorf("request error: %w", e.error).Error()
}

type InvalidContentTypeError struct {
	error       error
	contentType string
}

func (e *InvalidContentTypeError) Error() string {
	return fmt.Errorf("invalid content type %q: %w", e.contentType, e.error).Error()
}

func (e *InvalidContentTypeError) ContentType() string {
	return e.contentType
}
