-- name: CreateMembership :one
INSERT INTO memberships (id, user_id, group_id, roles, status, joined_via)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetMembership :one
SELECT * FROM memberships WHERE user_id = $1 AND group_id = $2;

-- name: GetMembershipByID :one
SELECT * FROM memberships WHERE id = $1;

-- name: ListMembers :many
SELECT sqlc.embed(memberships), sqlc.embed(users)
FROM memberships
JOIN users ON users.id = memberships.user_id
WHERE memberships.group_id = $1 AND users.deleted_at IS NULL
ORDER BY memberships.joined_at;

-- name: UpdateMembershipRoles :one
UPDATE memberships SET roles = $3 WHERE user_id = $1 AND group_id = $2 RETURNING *;

-- name: UpdateMembershipStatus :one
UPDATE memberships SET status = $3 WHERE user_id = $1 AND group_id = $2 RETURNING *;

-- name: CountActiveMembers :one
SELECT count(*) FROM memberships WHERE group_id = $1 AND status = 'ACTIVE';

-- name: ListActiveMemberUserIDs :many
SELECT user_id FROM memberships WHERE group_id = $1 AND status = 'ACTIVE';
