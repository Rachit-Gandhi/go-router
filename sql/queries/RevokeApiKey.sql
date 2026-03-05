-- name: RevokeApiKey :one
UPDATE api_keys
SET disabled = true,
    disabled_at = now()
WHERE id = $1 AND user_id = $2 AND deleted = false
RETURNING *;
