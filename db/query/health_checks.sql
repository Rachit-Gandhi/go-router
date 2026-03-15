-- name: CreateHealthCheck :one
INSERT INTO health_checks (service)
VALUES ($1)
RETURNING id, service, checked_at;

-- name: ListHealthChecks :many
SELECT id, service, checked_at
FROM health_checks
ORDER BY checked_at DESC;
