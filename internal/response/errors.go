package response

import (
	"encoding/json"
	"os"
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
		resp.Details = detail.Error()
	}
	_ = w.WriteJSON(status, resp)
	if status == 429 {
		writeStatusFile(trace)
	}
}

func writeStatusFile(trace string) {
	now := time.Now().UTC().Format(time.RFC3339)
	payload := StatusFile{LastRateLimit: &trace, UpdatedAt: &now}
	data, _ := json.MarshalIndent(payload, "", "  ")
	_ = os.WriteFile("status.json", data, 0644)
}
