package config

import "os"

// EnvOrDefault returns the environment variable value or fallback when unset.
func EnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
