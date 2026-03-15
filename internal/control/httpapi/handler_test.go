package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Rachit-Gandhi/go-router/internal/auth"
)

func TestHealthz(t *testing.T) {
	tc := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/control/healthz", nil)
	rec := httptest.NewRecorder()

	tc.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if got := rec.Body.String(); got != `{"status":"ok"}` {
		t.Fatalf("expected body %q, got %q", `{"status":"ok"}`, got)
	}
}

func TestHealthzMethodNotAllowed(t *testing.T) {
	tc := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/control/healthz", nil)
	rec := httptest.NewRecorder()

	tc.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestNewHandlerWithPostgresFromEnvRequiresSessionSecret(t *testing.T) {
	t.Setenv("CONTROL_DB_DSN", "postgres://invalid")
	t.Setenv("CONTROL_SESSION_SECRET", "")
	t.Setenv("CONTROL_PROVIDER_KEY_SECRET", "provider-secret")

	h, db, err := NewHandlerWithPostgresFromEnv(time.Now)
	if err == nil || !strings.Contains(err.Error(), "CONTROL_SESSION_SECRET is required") {
		t.Fatalf("expected CONTROL_SESSION_SECRET required error, got handler=%v db=%v err=%v", h, db, err)
	}
}

func TestNewHandlerWithPostgresFromEnvRequiresProviderKeySecret(t *testing.T) {
	t.Setenv("CONTROL_DB_DSN", "postgres://invalid")
	t.Setenv("CONTROL_SESSION_SECRET", "session-secret")
	t.Setenv("CONTROL_PROVIDER_KEY_SECRET", "")

	h, db, err := NewHandlerWithPostgresFromEnv(time.Now)
	if err == nil || !strings.Contains(err.Error(), "CONTROL_PROVIDER_KEY_SECRET is required") {
		t.Fatalf("expected CONTROL_PROVIDER_KEY_SECRET required error, got handler=%v db=%v err=%v", h, db, err)
	}
}

func TestNewHandlerWithPostgresFromEnvRejectsInvalidInsecureCookiesFlag(t *testing.T) {
	t.Setenv("CONTROL_DB_DSN", "postgres://invalid")
	t.Setenv("CONTROL_SESSION_SECRET", "session-secret")
	t.Setenv("CONTROL_PROVIDER_KEY_SECRET", "provider-secret")
	t.Setenv("CONTROL_ALLOW_INSECURE_COOKIES", "not-a-bool")

	h, db, err := NewHandlerWithPostgresFromEnv(time.Now)
	if err == nil || !strings.Contains(err.Error(), "invalid CONTROL_ALLOW_INSECURE_COOKIES value") {
		t.Fatalf("expected invalid insecure-cookies error, got handler=%v db=%v err=%v", h, db, err)
	}
}

func TestNewMagicLinkSenderFromEnvDefaultsToFileDelivery(t *testing.T) {
	t.Setenv("CONTROL_SMTP_HOST", "")
	t.Setenv("CONTROL_MAGIC_LINK_LOG_PATH", "")

	sender, err := newMagicLinkSenderFromEnv()
	if err != nil {
		t.Fatalf("newMagicLinkSenderFromEnv error: %v", err)
	}
	if got := magicLinkDelivery(sender); got != "file" {
		t.Fatalf("expected file delivery mode, got %q", got)
	}
	if !shouldExposeMagicLinkCode(sender) {
		t.Fatalf("expected file sender to expose debug code")
	}
}

func TestNewMagicLinkSenderFromEnvRejectsInvalidSMTPPort(t *testing.T) {
	t.Setenv("CONTROL_SMTP_HOST", "smtp.example.com")
	t.Setenv("CONTROL_SMTP_PORT", "abc")
	t.Setenv("CONTROL_SMTP_FROM", "no-reply@example.com")

	sender, err := newMagicLinkSenderFromEnv()
	if err == nil || !strings.Contains(err.Error(), "CONTROL_SMTP_PORT must be a positive integer") {
		t.Fatalf("expected invalid smtp port error, got sender=%v err=%v", sender, err)
	}
}

func TestSetSessionCookieSecureAttributeRespectsConfig(t *testing.T) {
	codec, err := auth.NewSessionCodec("test-control-session-secret")
	if err != nil {
		t.Fatalf("new session codec: %v", err)
	}
	claims := auth.SessionClaims{
		OrgID:          "org_123",
		UserID:         "usr_123",
		RefreshTokenID: "rt_123",
		ExpiresAtUnix:  time.Now().Add(sessionTTL).Unix(),
	}
	expiresAt := time.Now().Add(sessionTTL)

	t.Run("secure cookies enabled", func(t *testing.T) {
		h := &controlHandler{
			sessions:      codec,
			secureCookies: true,
		}
		rec := httptest.NewRecorder()
		if err := h.setSessionCookie(rec, claims, expiresAt); err != nil {
			t.Fatalf("set session cookie: %v", err)
		}

		if got := rec.Header().Get("Set-Cookie"); !strings.Contains(got, "Secure") {
			t.Fatalf("expected Secure attribute in Set-Cookie, got %q", got)
		}
	})

	t.Run("secure cookies disabled", func(t *testing.T) {
		h := &controlHandler{
			sessions:      codec,
			secureCookies: false,
		}
		rec := httptest.NewRecorder()
		if err := h.setSessionCookie(rec, claims, expiresAt); err != nil {
			t.Fatalf("set session cookie: %v", err)
		}

		if got := rec.Header().Get("Set-Cookie"); strings.Contains(got, "Secure") {
			t.Fatalf("expected no Secure attribute in Set-Cookie, got %q", got)
		}
	})
}
