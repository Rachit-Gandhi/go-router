-- name: GetApiKeys :many
SELECT name,api_key_show_string,disabled,deleted,last_used_at,disabled_at,deleted_at FROM api_keys 
WHERE user_id = $1
ORDER BY last_used_at DESC;