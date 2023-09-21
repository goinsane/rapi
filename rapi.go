package rapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/goinsane/logng"
)

type HandleFunc func(in interface{}, send SendFunc)
type SendFunc func(out interface{}, code int)

type Handler struct {
	Logger *logng.Logger
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, in interface{}, fn HandleFunc) {
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(r.Body)

	send := func(out interface{}, code int) {
		data, err := json.Marshal(out)
		if err != nil {
			h.Logger.Errorf("unable to marshal json: %w", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		data = append(data, '\n')
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
		w.WriteHeader(code)
		_, err = io.Copy(w, bytes.NewBuffer(data))
		if err != nil {
			h.Logger.Errorf("unable to write data: %w", err)
			return
		}
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		h.Logger.Errorf("unable to read data: %w", err)
		send(&errorResponse{Error: "read data error"}, http.StatusBadRequest)
		return
	}

	p := makePtr(in)

	err = json.Unmarshal(data, p)
	if err != nil {
		h.Logger.Errorf("unable to unmarshal json: %w", err)
		send(&errorResponse{Error: "json unmarshal error"}, http.StatusBadRequest)
		return
	}

	fn(p, send)
}
