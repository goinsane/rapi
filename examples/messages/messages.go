package messages

import "time"

type ErrorResponse struct {
	ErrorMsg string `json:"error"`
}

func (r *ErrorResponse) Error() string {
	return r.ErrorMsg
}

type PingRequest struct {
	Payload string
}

type PingReply struct {
	Payload string
}

type TestRequest struct {
	X int
	T time.Time
	B []byte
}

type TestReply struct {
	X  int
	T  time.Time
	B  []byte
	BS string
	D  time.Duration
}
