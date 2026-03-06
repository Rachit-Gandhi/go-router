-- name: UpdateTransactionStatus :one
UPDATE razorpay_transactions
SET status = $2, completed_at = now()
WHERE transaction_id = $1
RETURNING *;
