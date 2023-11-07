package messages

import "time"

type ErrorReply struct {
	ErrorMsg string `json:"error"`
}

func (r *ErrorReply) Error() string {
	return r.ErrorMsg
}

type PingRequest struct {
	Payload string
}

type PingReply struct {
	Payload string
}

type ReverseRequest struct {
	String string `json:"string"`
}

type ReverseReply struct {
	ReversedString string `json:"reversedString"`
}

type NowRequest struct {
	Time  *time.Time    `json:"time"`
	Drift time.Duration `json:"drift"`
}

type NowReply struct {
	Now time.Time `json:"now"`
}
