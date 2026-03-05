package response

import (
	"encoding/json"
	"net/http"
)

type JsonWriter interface {
	WriteJSON(status int, v any) error
}

type HTTPWriter struct {
	w http.ResponseWriter
}

func Wrap(w http.ResponseWriter) *HTTPWriter {
	return &HTTPWriter{w: w}
}

func (h *HTTPWriter) WriteJSON(status int, v any) error {
	h.w.Header().Set("Content-Type", "application/json")
	h.w.WriteHeader(status)
	return json.NewEncoder(h.w).Encode(v)
}
