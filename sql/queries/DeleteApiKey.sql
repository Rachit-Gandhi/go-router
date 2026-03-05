-- name: DeleteApiKey :one
UPDATE api_keys
SET deleted = true,
    deleted_at = now()
WHERE id = $1 AND user_id = $2 AND deleted = false
RETURNING *;
