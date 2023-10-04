package messages

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
}

type TestReply struct {
	X int
}
