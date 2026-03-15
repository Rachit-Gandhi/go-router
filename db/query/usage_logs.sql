-- name: CreateUsageLog :one
INSERT INTO usage_logs (
  org_id,
  team_id,
  user_id,
  api_key_id,
  provider,
  model,
  request_tokens,
  response_tokens,
  latency_ms,
  status_code,
  request_fingerprint
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, org_id, team_id, user_id, api_key_id, provider, model, request_tokens, response_tokens, latency_ms, status_code, request_fingerprint, created_at;
