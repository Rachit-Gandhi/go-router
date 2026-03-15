package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthz(t *testing.T) {
	h := NewHandlerWithoutDB(time.Now)

	req := httptest.NewRequest(http.MethodGet, "/v1/router/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if got := rec.Body.String(); got != `{"status":"ok"}` {
		t.Fatalf("expected body %q, got %q", `{"status":"ok"}`, got)
	}
}

func TestHealthzMethodNotAllowed(t *testing.T) {
	h := NewHandlerWithoutDB(time.Now)

	req := httptest.NewRequest(http.MethodPost, "/v1/router/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestNewHandlerWithPostgresFromEnvRequiresDSN(t *testing.T) {
	t.Setenv("ROUTER_DB_DSN", "")

	h, db, err := NewHandlerWithPostgresFromEnv(time.Now)
	if err == nil || !strings.Contains(err.Error(), "ROUTER_DB_DSN is required") {
		t.Fatalf("expected ROUTER_DB_DSN required error, got handler=%v db=%v err=%v", h, db, err)
	}
}

func TestRequestFingerprintDistinguishesDelimiterCollisionCases(t *testing.T) {
	singleMessage := chatCompletionsRequest{
		Model: "auto",
		Messages: []chatMessage{
			{Role: "user", Content: "hello|assistant:world"},
		},
	}
	splitMessages := chatCompletionsRequest{
		Model: "auto",
		Messages: []chatMessage{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "world"},
		},
	}

	singleFingerprint := requestFingerprint(singleMessage)
	splitFingerprint := requestFingerprint(splitMessages)
	if singleFingerprint == splitFingerprint {
		t.Fatalf("expected canonical fingerprinting to avoid delimiter collisions")
	}
}

func TestRequestFingerprintNormalizesWhitespace(t *testing.T) {
	withWhitespace := chatCompletionsRequest{
		Model: "  auto  ",
		Messages: []chatMessage{
			{Role: " user ", Content: "  hello there  "},
		},
	}
	normalized := chatCompletionsRequest{
		Model: "auto",
		Messages: []chatMessage{
			{Role: "user", Content: "hello there"},
		},
	}

	if requestFingerprint(withWhitespace) != requestFingerprint(normalized) {
		t.Fatalf("expected fingerprinting to normalize trimmed request fields")
	}
}
