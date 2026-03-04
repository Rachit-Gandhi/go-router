-- name: CreateApiKey :one
INSERT INTO api_keys (user_id,name,key_hash,api_key_show_string)
VALUES(
    $1,
    $2,
    $3,
    $4
)
RETURNING *;