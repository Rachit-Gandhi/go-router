-- name: GetUserBalance :one
SELECT balance_credits_int FROM users WHERE user_id = $1;
