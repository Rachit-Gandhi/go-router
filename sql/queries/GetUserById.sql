-- name: GetUserByID :one
SELECT user_id, username, email, password, balance_credits_int FROM users WHERE user_id = $1;