package rapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/goinsane/logng"
)

func sendResponse(logger *logng.Logger, w http.ResponseWriter, out interface{}, code int) {
	data, err := json.Marshal(out)
	if err != nil {
		logger.Errorf("unable to marshal output: %w", err)
		data, _ := json.Marshal(http.StatusText(http.StatusInternalServerError))
		data = append(data, '\n')
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
		w.WriteHeader(http.StatusInternalServerError)
		_, err = io.Copy(w, bytes.NewBuffer(data))
		if err != nil {
			logger.Errorf("unable to write response body: %w", err)
			return
		}
		return
	}
	data = append(data, '\n')
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
	w.WriteHeader(code)
	_, err = io.Copy(w, bytes.NewBuffer(data))
	if err != nil {
		logger.Errorf("unable to write response body: %w", err)
		return
	}
}
