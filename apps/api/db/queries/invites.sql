-- name: CreateInvite :one
INSERT INTO invites (id, group_id, code, roles, expires_at, max_uses, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetInviteByCode :one
SELECT * FROM invites WHERE code = $1;

-- name: ListInvitesByGroup :many
SELECT * FROM invites WHERE group_id = $1 AND revoked_at IS NULL ORDER BY created_at DESC;

-- name: IncrementInviteUses :one
UPDATE invites SET uses = uses + 1 WHERE id = $1 RETURNING *;

-- name: RevokeInvite :exec
UPDATE invites SET revoked_at = now() WHERE id = $1 AND group_id = $2;
