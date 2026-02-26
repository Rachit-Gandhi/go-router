-- name: CreateApiKey :one
INSERT INTO api_keys (user_id,name,key_hash)
VALUES(
    $1,
    $2,
    $3
)
RETURNING *;