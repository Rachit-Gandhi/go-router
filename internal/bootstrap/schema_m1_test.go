package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreSchemaMigrationExistsWithV1Tables(t *testing.T) {
	migrationPath := filepath.Clean("../../db/migrations/000003_v1_core_schema.sql")
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("expected migration file at %s: %v", migrationPath, err)
	}

	got := string(content)
	requiredSnippets := []string{
		"CREATE TABLE IF NOT EXISTS users",
		"CREATE TABLE IF NOT EXISTS orgs",
		"CREATE TABLE IF NOT EXISTS org_memberships",
		"CREATE TABLE IF NOT EXISTS teams",
		"CREATE TABLE IF NOT EXISTS team_memberships",
		"CREATE TABLE IF NOT EXISTS team_admin_scopes",
		"CREATE TABLE IF NOT EXISTS auth_magic_links",
		"CREATE TABLE IF NOT EXISTS auth_refresh_tokens",
		"CREATE TABLE IF NOT EXISTS user_team_api_keys",
		"CREATE TABLE IF NOT EXISTS org_provider_keys",
		"CREATE TABLE IF NOT EXISTS org_model_policies",
		"CREATE TABLE IF NOT EXISTS team_model_policies",
		"CREATE TABLE IF NOT EXISTS usage_logs",
		"REFERENCES orgs(id)",
	}
	for _, snippet := range requiredSnippets {
		snippet := snippet
		t.Run(snippet, func(t *testing.T) {
			if !strings.Contains(got, snippet) {
				t.Fatalf("expected migration to include %q", snippet)
			}
		})
	}
}

func TestCoreSQLCQuerySpecsExist(t *testing.T) {
	queryFiles := []string{
		"../../db/query/orgs.sql",
		"../../db/query/teams.sql",
		"../../db/query/memberships.sql",
		"../../db/query/auth.sql",
		"../../db/query/api_keys.sql",
		"../../db/query/policies.sql",
		"../../db/query/router_lookup.sql",
		"../../db/query/usage_logs.sql",
	}

	for _, file := range queryFiles {
		file := file
		t.Run(file, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Clean(file))
			if err != nil {
				t.Fatalf("expected query file %s: %v", file, err)
			}
			if !strings.Contains(string(content), "-- name:") {
				t.Fatalf("expected sqlc named query declarations in %s", file)
			}
		})
	}
}
