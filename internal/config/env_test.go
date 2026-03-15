package config

import "testing"

func TestEnvOrDefaultReturnsEnvValue(t *testing.T) {
	t.Setenv("TEST_ENV_OR_DEFAULT", "set-value")

	got := EnvOrDefault("TEST_ENV_OR_DEFAULT", "fallback")
	if got != "set-value" {
		t.Fatalf("expected env value, got %q", got)
	}
}

func TestEnvOrDefaultReturnsFallback(t *testing.T) {
	t.Setenv("TEST_ENV_OR_DEFAULT", "")

	got := EnvOrDefault("TEST_ENV_OR_DEFAULT", "fallback")
	if got != "fallback" {
		t.Fatalf("expected fallback value, got %q", got)
	}
}
