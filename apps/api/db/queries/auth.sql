-- name: CreateIdentity :one
INSERT INTO auth_identities (id, user_id, provider, provider_user_id, email)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetIdentityByProvider :one
SELECT * FROM auth_identities WHERE provider = $1 AND provider_user_id = $2;

-- name: ListIdentitiesByUser :many
SELECT * FROM auth_identities WHERE user_id = $1 ORDER BY created_at;

-- name: CreateDevice :one
INSERT INTO devices (id, user_id, platform, name)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetDevice :one
SELECT * FROM devices WHERE id = $1 AND user_id = $2;

-- name: ListDevicesByUser :many
SELECT * FROM devices WHERE user_id = $1 ORDER BY last_seen_at DESC;

-- name: TouchDevice :exec
UPDATE devices SET last_seen_at = now() WHERE id = $1;

-- name: UpdateDevicePush :one
UPDATE devices
SET push_provider = $3, push_token = $4, push_subscription = $5, enabled = true
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DisableDevicePush :exec
UPDATE devices SET enabled = false WHERE id = $1;

-- name: DeleteDevice :exec
DELETE FROM devices WHERE id = $1 AND user_id = $2;

-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (id, user_id, device_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT * FROM refresh_tokens WHERE token_hash = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET revoked_at = now(), replaced_by = sqlc.narg(replaced_by)
WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeAllUserRefreshTokens :exec
UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL;

-- name: DeleteExpiredRefreshTokens :execrows
DELETE FROM refresh_tokens WHERE expires_at < now() - interval '7 days';
