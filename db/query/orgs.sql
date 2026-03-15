-- name: CreateUser :one
INSERT INTO users (id, email, name)
VALUES ($1, $2, $3)
RETURNING id, email, name, created_at, updated_at;

-- name: GetUserByEmail :one
SELECT id, email, name, created_at, updated_at
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, name, created_at, updated_at
FROM users
WHERE id = $1;

-- name: CreateOrg :one
INSERT INTO orgs (id, name, owner_user_id)
VALUES ($1, $2, $3)
RETURNING id, name, owner_user_id, created_at, updated_at;

-- name: GetOrgByID :one
SELECT id, name, owner_user_id, created_at, updated_at
FROM orgs
WHERE id = $1;
