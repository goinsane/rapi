package rapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
)

func sendResponse(h *Handler, w http.ResponseWriter, out interface{}, header http.Header, code int) {
	data, err := json.Marshal(out)
	if err != nil {
		h.Logger.Errorf("unable to marshal response body to json: %w", err)
		data, _ := json.Marshal(http.StatusText(http.StatusInternalServerError))
		data = append(data, '\n')
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
		w.WriteHeader(http.StatusInternalServerError)
		_, err = io.Copy(w, bytes.NewBuffer(data))
		if err != nil {
			h.Logger.Errorf("unable to write response body: %w", err)
			return
		}
		return
	}
	data = append(data, '\n')
	for key, val := range header {
		w.Header()[key] = val
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
	w.WriteHeader(code)
	_, err = io.Copy(w, bytes.NewBuffer(data))
	if err != nil {
		h.Logger.Errorf("unable to write response body: %w", err)
		return
	}
}
