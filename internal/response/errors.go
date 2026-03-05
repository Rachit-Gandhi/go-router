package response

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	TraceID string `json:"trace_id"`
	Details string `json:"details,omitempty"`
}

type StatusFile struct {
	LastRateLimit *string `json:"last_rate_limit"`
	UpdatedAt     *string `json:"updated_at"`
}

func WriteError(w JsonWriter, status int, msg string, detail error) {
	trace := uuid.NewString()
	resp := ErrorResponse{Error: msg, TraceID: trace}
	if detail != nil {
		log.Printf("trace_id=%s error=%v", trace, detail)
		if status == http.StatusBadRequest {
			resp.Details = detail.Error()
		}
	}
	_ = w.WriteJSON(status, resp)
	if status == http.StatusTooManyRequests {
		writeStatusFile(trace)
	}
}

func writeStatusFile(trace string) {
	now := time.Now().UTC().Format(time.RFC3339)
	payload := StatusFile{LastRateLimit: &trace, UpdatedAt: &now}
	data, _ := json.MarshalIndent(payload, "", "  ")

	path := os.Getenv("STATUS_PATH")
	if path == "" {
		path = filepath.Join(os.TempDir(), "go-router-status.json")
	}
	_ = os.WriteFile(path, data, 0644)
}
