-- name: CreateMagicLink :one
INSERT INTO auth_magic_links (id, org_id, email, code_hash, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, org_id, email, code_hash, expires_at, consumed_at, created_at;

-- name: ConsumeMagicLink :one
UPDATE auth_magic_links
SET consumed_at = NOW()
WHERE id = $1
  AND consumed_at IS NULL
  AND expires_at > NOW()
RETURNING id, org_id, email, code_hash, expires_at, consumed_at, created_at;

-- name: CreateRefreshToken :one
INSERT INTO auth_refresh_tokens (id, org_id, user_id, token_hash, session_id, device_info, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, org_id, user_id, token_hash, session_id, device_info, expires_at, revoked_at, last_used_at, created_at;

-- name: GetRefreshTokenByID :one
SELECT id, org_id, user_id, token_hash, session_id, device_info, expires_at, revoked_at, last_used_at, created_at
FROM auth_refresh_tokens
WHERE id = $1;

-- name: RevokeRefreshToken :execrows
UPDATE auth_refresh_tokens
SET revoked_at = NOW()
WHERE id = $1 AND revoked_at IS NULL;

-- name: TouchRefreshToken :execrows
UPDATE auth_refresh_tokens
SET last_used_at = NOW()
WHERE id = $1
  AND revoked_at IS NULL
  AND expires_at > NOW();
