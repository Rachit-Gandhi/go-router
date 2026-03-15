-- name: CreateTeam :one
INSERT INTO teams (id, org_id, name, profile_jsonb, rate_limit_per_minute)
VALUES ($1, $2, $3, COALESCE($4, '{}'::jsonb), $5)
RETURNING id, org_id, name, profile_jsonb, rate_limit_per_minute, created_at, updated_at;

-- name: GetTeamByID :one
SELECT id, org_id, name, profile_jsonb, rate_limit_per_minute, created_at, updated_at
FROM teams
WHERE id = $1 AND org_id = $2;

-- name: ListTeamsByOrg :many
SELECT id, org_id, name, profile_jsonb, rate_limit_per_minute, created_at, updated_at
FROM teams
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT $2;
