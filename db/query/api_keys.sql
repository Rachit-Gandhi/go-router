-- name: CreateUserTeamAPIKey :one
INSERT INTO user_team_api_keys (id, org_id, team_id, user_id, key_hash, key_prefix, is_active)
VALUES ($1, $2, $3, $4, $5, $6, TRUE)
RETURNING id, org_id, team_id, user_id, key_hash, key_prefix, revoked_at, is_active, last_used_at, created_at;

-- name: GetActiveUserTeamAPIKeyByHash :one
SELECT id, org_id, team_id, user_id, key_hash, key_prefix, revoked_at, is_active, last_used_at, created_at
FROM user_team_api_keys
WHERE key_hash = $1
  AND is_active = TRUE
  AND revoked_at IS NULL;

-- name: RevokeUserTeamAPIKey :execrows
UPDATE user_team_api_keys
SET is_active = FALSE,
    revoked_at = NOW()
WHERE id = $1
  AND org_id = $2
  AND revoked_at IS NULL;

-- name: ListUserTeamAPIKeys :many
SELECT id, org_id, team_id, user_id, key_hash, key_prefix, revoked_at, is_active, last_used_at, created_at
FROM user_team_api_keys
WHERE org_id = $1 AND team_id = $2 AND user_id = $3
ORDER BY created_at DESC;
