-- name: ResolveIdentityByAPIKeyHash :one
SELECT id, org_id, team_id, user_id, key_prefix, revoked_at, is_active, last_used_at, created_at
FROM user_team_api_keys
WHERE key_hash = $1
  AND is_active = TRUE
  AND revoked_at IS NULL;

-- name: ListEffectiveAllowedModels :many
SELECT o.provider, o.model
FROM org_model_policies AS o
LEFT JOIN team_model_policies AS t
  ON t.org_id = o.org_id
 AND t.team_id = $2
 AND t.provider = o.provider
 AND t.model = o.model
WHERE o.org_id = $1
  AND o.is_allowed = TRUE
  AND COALESCE(t.is_allowed, TRUE) = TRUE
ORDER BY o.provider, o.model
LIMIT sqlc.arg(limit_rows)
OFFSET COALESCE(sqlc.narg(offset_rows)::int, 0);
