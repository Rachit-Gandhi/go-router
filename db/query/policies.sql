-- name: UpsertOrgModelPolicy :one
INSERT INTO org_model_policies (org_id, provider, model, is_allowed)
VALUES ($1, $2, $3, $4)
ON CONFLICT (org_id, provider, model)
DO UPDATE SET is_allowed = EXCLUDED.is_allowed
RETURNING org_id, provider, model, is_allowed, created_at;

-- name: UpsertTeamModelPolicy :one
INSERT INTO team_model_policies (org_id, team_id, provider, model, is_allowed)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (org_id, team_id, provider, model)
DO UPDATE SET is_allowed = EXCLUDED.is_allowed
RETURNING org_id, team_id, provider, model, is_allowed, created_at;

-- name: ListOrgAllowedModels :many
SELECT org_id, provider, model, is_allowed, created_at
FROM org_model_policies
WHERE org_id = $1 AND is_allowed = TRUE
ORDER BY provider, model;

-- name: ListTeamModelPolicies :many
SELECT org_id, team_id, provider, model, is_allowed, created_at
FROM team_model_policies
WHERE org_id = $1 AND team_id = $2
ORDER BY provider, model;
