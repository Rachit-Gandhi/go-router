package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreSchemaHasOwnershipDeleteBehavior(t *testing.T) {
	migrationPath := filepath.Clean("../../db/migrations/000003_v1_core_schema.sql")
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("expected migration file at %s: %v", migrationPath, err)
	}

	got := string(content)
	if !strings.Contains(got, "owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT") {
		t.Fatal("expected orgs.owner_user_id to declare explicit ON DELETE RESTRICT behavior")
	}
}

func TestCoreSchemaRemovesRedundantTeamMembershipUniqueness(t *testing.T) {
	migrationPath := filepath.Clean("../../db/migrations/000003_v1_core_schema.sql")
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("expected migration file at %s: %v", migrationPath, err)
	}

	got := string(content)
	if strings.Contains(got, "UNIQUE (org_id, team_id, user_id)") {
		t.Fatal("expected redundant UNIQUE (org_id, team_id, user_id) to be removed from team_memberships")
	}
}

func TestCoreSchemaHasKeyHashAndUsageLogIndexes(t *testing.T) {
	migrationPath := filepath.Clean("../../db/migrations/000003_v1_core_schema.sql")
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("expected migration file at %s: %v", migrationPath, err)
	}

	got := string(content)
	requiredSnippets := []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_user_team_api_keys_key_hash_unique",
		"CREATE INDEX IF NOT EXISTS idx_usage_logs_org_created_at ON usage_logs (org_id, created_at DESC);",
		"CREATE INDEX IF NOT EXISTS idx_usage_logs_team_created_at ON usage_logs (team_id, created_at DESC);",
		"CREATE INDEX IF NOT EXISTS idx_usage_logs_request_fingerprint ON usage_logs (request_fingerprint);",
		"PARTITION BY RANGE (created_at)",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(got, snippet) {
			t.Fatalf("expected migration to include %q", snippet)
		}
	}
}

func TestUsageLogsPartitionFollowupMigrationExists(t *testing.T) {
	migrationPath := filepath.Clean("../../db/migrations/000004_usage_logs_partition_maintenance.sql")
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("expected follow-up migration file at %s: %v", migrationPath, err)
	}

	got := string(content)
	requiredSnippets := []string{
		"format('usage_logs_%s'",
		"PARTITION OF usage_logs",
		"FOR VALUES FROM",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(got, snippet) {
			t.Fatalf("expected follow-up migration to include %q", snippet)
		}
	}
}

func TestUsageLogsPartitionDownUsesCatalogDiscovery(t *testing.T) {
	migrationPath := filepath.Clean("../../db/migrations/000004_usage_logs_partition_maintenance.sql")
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("expected follow-up migration file at %s: %v", migrationPath, err)
	}

	got := string(content)
	downMarker := "-- +goose Down"
	downStart := strings.Index(got, downMarker)
	if downStart == -1 {
		t.Fatalf("expected migration to include %q", downMarker)
	}

	downBlock := got[downStart:]
	requiredSnippets := []string{
		"FROM pg_inherits",
		"inhparent = 'usage_logs'::regclass",
		"partition_name TEXT;",
		"DROP TABLE IF EXISTS %I;",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(downBlock, snippet) {
			t.Fatalf("expected usage_logs down migration to include %q", snippet)
		}
	}
	if strings.Contains(downBlock, "date_trunc('month', NOW())") {
		t.Fatal("expected usage_logs down migration to avoid NOW-based partition name calculation")
	}
}

func TestAPIKeyQueriesDoNotProjectKeyHashInCreateOrList(t *testing.T) {
	queryPath := filepath.Clean("../../db/query/api_keys.sql")
	content, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatalf("expected query file at %s: %v", queryPath, err)
	}

	got := string(content)
	if strings.Contains(got, "RETURNING id, org_id, team_id, user_id, key_hash") {
		t.Fatal("expected CreateUserTeamAPIKey to avoid returning key_hash")
	}
	if strings.Contains(got, "SELECT id, org_id, team_id, user_id, key_hash") {
		t.Fatal("expected list/get projections to avoid selecting key_hash")
	}
}

func TestAuthTouchRefreshTokenIsBoundedToActiveTokens(t *testing.T) {
	queryPath := filepath.Clean("../../db/query/auth.sql")
	content, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatalf("expected query file at %s: %v", queryPath, err)
	}

	got := string(content)
	requiredSnippets := []string{
		"AND revoked_at IS NULL",
		"AND expires_at > NOW()",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(got, snippet) {
			t.Fatalf("expected TouchRefreshToken to include %q", snippet)
		}
	}
}

func TestRefreshTokenQueriesAreScopedToOrg(t *testing.T) {
	queryPath := filepath.Clean("../../db/query/auth.sql")
	content, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatalf("expected query file at %s: %v", queryPath, err)
	}

	got := string(content)
	cases := []struct {
		queryName string
		snippet   string
	}{
		{queryName: "GetRefreshTokenByID", snippet: "WHERE id = $1 AND org_id = $2"},
		{queryName: "RevokeRefreshToken", snippet: "WHERE id = $1 AND org_id = $2 AND revoked_at IS NULL"},
		{queryName: "TouchRefreshToken", snippet: "AND org_id = $2"},
	}

	for _, tc := range cases {
		queryBlock := extractNamedQueryBlock(t, got, tc.queryName)
		if !strings.Contains(queryBlock, tc.snippet) {
			t.Fatalf("expected %s to include %q", tc.queryName, tc.snippet)
		}
	}
}

func TestListTeamAdminScopesIsBounded(t *testing.T) {
	queryPath := filepath.Clean("../../db/query/memberships.sql")
	content, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatalf("expected query file at %s: %v", queryPath, err)
	}

	got := string(content)
	if !strings.Contains(got, "LIMIT $3") {
		t.Fatal("expected ListTeamAdminScopes to include LIMIT $3")
	}
}

func TestListEffectiveAllowedModelsIsBoundedAndIndexedByPK(t *testing.T) {
	queryPath := filepath.Clean("../../db/query/router_lookup.sql")
	content, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatalf("expected query file at %s: %v", queryPath, err)
	}

	queryBlock := extractNamedQueryBlock(t, string(content), "ListEffectiveAllowedModels")
	requiredQuerySnippets := []string{
		"LIMIT sqlc.arg(limit_rows)",
		"OFFSET COALESCE(sqlc.narg(offset_rows)::int, 0)",
		"ORDER BY o.provider, o.model",
	}
	for _, snippet := range requiredQuerySnippets {
		if !strings.Contains(queryBlock, snippet) {
			t.Fatalf("expected ListEffectiveAllowedModels to include %q", snippet)
		}
	}

	migrationPath := filepath.Clean("../../db/migrations/000003_v1_core_schema.sql")
	migrationContent, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("expected migration file at %s: %v", migrationPath, err)
	}

	migration := string(migrationContent)
	requiredIndexCoverageSnippets := []string{
		"PRIMARY KEY (org_id, provider, model)",
		"PRIMARY KEY (org_id, team_id, provider, model)",
	}
	for _, snippet := range requiredIndexCoverageSnippets {
		if !strings.Contains(migration, snippet) {
			t.Fatalf("expected policy tables to include index coverage via %q", snippet)
		}
	}
}

func TestCreateTeamCoalescesProfileJSONB(t *testing.T) {
	queryPath := filepath.Clean("../../db/query/teams.sql")
	content, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatalf("expected query file at %s: %v", queryPath, err)
	}

	if !strings.Contains(string(content), "COALESCE($4, '{}'::jsonb)") {
		t.Fatal("expected CreateTeam to coalesce profile_jsonb parameter")
	}
}

func extractNamedQueryBlock(t *testing.T, content, queryName string) string {
	t.Helper()

	startMarker := "-- name: " + queryName
	start := strings.Index(content, startMarker)
	if start == -1 {
		t.Fatalf("expected query marker %q", startMarker)
	}

	queryBlock := content[start:]
	next := strings.Index(queryBlock[len(startMarker):], "-- name:")
	if next != -1 {
		queryBlock = queryBlock[:len(startMarker)+next]
	}
	return queryBlock
}
