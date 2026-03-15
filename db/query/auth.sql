-- name: CreateMagicLink :one
INSERT INTO auth_magic_links (id, org_id, email, code_hash, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, org_id, email, code_hash, expires_at, consumed_at, created_at;

-- name: CreateAuthLogin :one
INSERT INTO auth_logins (id, magic_link_id, org_id, user_id, role, email)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, magic_link_id, org_id, user_id, role, email, created_at;

-- name: GetAuthLoginByMagicLinkID :one
SELECT id, magic_link_id, org_id, user_id, role, email, created_at
FROM auth_logins
WHERE magic_link_id = $1;

-- name: ConsumeMagicLink :one
UPDATE auth_magic_links
SET consumed_at = NOW()
WHERE id = $1
  AND consumed_at IS NULL
  AND expires_at > NOW()
RETURNING id, org_id, email, code_hash, expires_at, consumed_at, created_at;

-- name: ConsumeMagicLinkByCode :one
UPDATE auth_magic_links
SET consumed_at = NOW()
WHERE id = $1
  AND code_hash = $2
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
WHERE id = $1 AND org_id = $2;

-- name: RevokeRefreshToken :execrows
UPDATE auth_refresh_tokens
SET revoked_at = NOW()
WHERE id = $1 AND org_id = $2 AND revoked_at IS NULL;

-- name: TouchRefreshToken :execrows
UPDATE auth_refresh_tokens
SET last_used_at = NOW()
WHERE id = $1
  AND org_id = $2
  AND revoked_at IS NULL
  AND expires_at > NOW();
