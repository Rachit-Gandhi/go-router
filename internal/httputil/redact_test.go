package httputil

import (
	"strings"
	"testing"
)

func TestRedactSecretsRedactsBearerAndAPIKeyFields(t *testing.T) {
	input := `Authorization: Bearer sk-live-secret-token {"api_key":"sk-provider-secret","token":"refresh-secret"}`
	redacted := RedactSecrets(input)

	for _, secret := range []string{"sk-live-secret-token", "sk-provider-secret", "refresh-secret"} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("expected secret %q to be redacted, got %q", secret, redacted)
		}
	}
	if !strings.Contains(redacted, "[REDACTED]") {
		t.Fatalf("expected redacted marker in output, got %q", redacted)
	}
}

func TestRedactSecretsRedactsSensitiveQueryParams(t *testing.T) {
	input := `POST /v1/control/orgs/org_1/providers/openai/keys?api_key=sk-query-secret&password=pw123`
	redacted := RedactSecrets(input)

	for _, secret := range []string{"sk-query-secret", "pw123"} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("expected secret %q to be redacted, got %q", secret, redacted)
		}
	}
}
