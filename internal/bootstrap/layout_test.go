package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLockedDirectoriesExist(t *testing.T) {
	requiredDirs := []string{
		"frontend",
		"cmd/control",
		"cmd/router",
		"internal/auth",
		"internal/rbac",
		"internal/control",
		"internal/router",
		"internal/store",
		"internal/crypto",
		"internal/policy",
		"db/migrations",
		"db/query",
	}

	for _, dir := range requiredDirs {
		info, err := os.Stat(filepath.Clean("../../" + dir))
		if err != nil {
			t.Fatalf("expected directory %s to exist: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", dir)
		}
	}
}

func TestSQLCToolingWired(t *testing.T) {
	sqlcConfig := filepath.Clean("../../db/sqlc.yaml")
	cfgContent, err := os.ReadFile(sqlcConfig)
	if err != nil {
		t.Fatalf("expected sqlc config at %s: %v", sqlcConfig, err)
	}
	if !strings.Contains(string(cfgContent), "version: \"2\"") {
		t.Fatalf("expected sqlc config to declare version 2")
	}

	migrationFile := filepath.Clean("../../db/migrations/000001_init.sql")
	migrationContent, err := os.ReadFile(migrationFile)
	if err != nil {
		t.Fatalf("expected migration file at %s: %v", migrationFile, err)
	}
	if !strings.Contains(string(migrationContent), "-- +goose Up") {
		t.Fatalf("expected goose up annotation in %s", migrationFile)
	}
	if !strings.Contains(string(migrationContent), "-- +goose Down") {
		t.Fatalf("expected goose down annotation in %s", migrationFile)
	}
}
