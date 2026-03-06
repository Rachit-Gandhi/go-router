-- name: AddUserCredits :one
UPDATE users
SET balance_credits_int = balance_credits_int + $2
WHERE user_id = $1
RETURNING user_id, username, email, password, balance_credits_int;
