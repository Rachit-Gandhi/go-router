-- name: CreateOrgProviderKey :one
INSERT INTO org_provider_keys (id, org_id, provider, key_ciphertext, key_nonce, key_kek_id, is_active)
VALUES ($1, $2, $3, $4, $5, $6, TRUE)
RETURNING id, org_id, provider, key_ciphertext, key_nonce, key_kek_id, is_active, created_at, updated_at;

-- name: GetOrgProviderKeyByID :one
SELECT id, org_id, provider, key_ciphertext, key_nonce, key_kek_id, is_active, created_at, updated_at
FROM org_provider_keys
WHERE id = $1;
