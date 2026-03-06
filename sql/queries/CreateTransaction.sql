-- name: CreateTransaction :one
INSERT INTO razorpay_transactions (user_id, amt, credits, status)
VALUES ($1, $2, $3, $4)
RETURNING *;
