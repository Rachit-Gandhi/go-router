package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFollowupMigrationAddsHealthCheckIndexes(t *testing.T) {
	migrationPath := filepath.Clean("../../db/migrations/000002_health_checks_indexes.sql")
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("expected follow-up migration at %s: %v", migrationPath, err)
	}

	got := string(content)
	requiredSnippets := []string{
		"CREATE INDEX IF NOT EXISTS idx_health_checks_service ON health_checks (service);",
		"CREATE INDEX IF NOT EXISTS idx_health_checks_checked_at_desc ON health_checks (checked_at DESC);",
		"DROP INDEX IF EXISTS idx_health_checks_checked_at_desc;",
		"DROP INDEX IF EXISTS idx_health_checks_service;",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(got, snippet) {
			t.Fatalf("expected migration to include %q", snippet)
		}
	}
}

func TestListHealthChecksIsBoundedAndStable(t *testing.T) {
	queryPath := filepath.Clean("../../db/query/health_checks.sql")
	content, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatalf("expected query file at %s: %v", queryPath, err)
	}

	got := string(content)
	if !strings.Contains(got, "ORDER BY checked_at DESC, id DESC") {
		t.Fatalf("expected stable ordering with checked_at and id tie-breaker")
	}
	if !strings.Contains(got, "LIMIT $1") {
		t.Fatalf("expected bounded query with LIMIT $1")
	}
}
