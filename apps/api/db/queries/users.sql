-- name: CreateUser :one
INSERT INTO users (id, name, email, password_hash, locale, timezone, secured_at, register_client_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetUserByRegisterClientID :one
SELECT * FROM users WHERE register_client_id = $1 AND deleted_at IS NULL;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE lower(email) = lower(sqlc.arg(email)) AND deleted_at IS NULL;

-- name: UpdateUserProfile :one
UPDATE users
SET name = $2, locale = $3, timezone = $4, settings = $5
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SetUserCredentials :one
UPDATE users
SET email = COALESCE(sqlc.narg(email), email),
    password_hash = COALESCE(sqlc.narg(password_hash), password_hash),
    secured_at = COALESCE(secured_at, now())
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: MarkUserSecured :exec
UPDATE users SET secured_at = COALESCE(secured_at, now()) WHERE id = $1;

-- name: SoftDeleteUser :exec
UPDATE users SET deleted_at = now() WHERE id = $1;
