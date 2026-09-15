-- name: CreateGroup :one
INSERT INTO groups (id, name, slug, kind, join_policy, join_code, media_mode, settings, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetGroup :one
SELECT * FROM groups WHERE id = $1;

-- name: GetGroupBySlug :one
SELECT * FROM groups WHERE slug = $1;

-- name: GetGroupByJoinCode :one
SELECT * FROM groups WHERE join_code = $1 AND archived_at IS NULL;

-- name: ListGroupsForUser :many
SELECT sqlc.embed(groups), sqlc.embed(memberships)
FROM groups
JOIN memberships ON memberships.group_id = groups.id
WHERE memberships.user_id = $1
  AND memberships.status = 'ACTIVE'
  AND groups.archived_at IS NULL
ORDER BY groups.created_at;

-- name: UpdateGroup :one
UPDATE groups
SET name = $2, kind = $3, join_policy = $4, public_read = $5, media_mode = $6, settings = $7
WHERE id = $1
RETURNING *;

-- name: SetGroupJoinCode :one
UPDATE groups SET join_code = $2 WHERE id = $1 RETURNING *;

-- name: ArchiveGroup :exec
UPDATE groups SET archived_at = now() WHERE id = $1;

-- name: NextGroupSeq :one
UPDATE groups SET last_seq = last_seq + 1 WHERE id = $1 RETURNING last_seq;

-- name: GroupSlugExists :one
SELECT EXISTS (SELECT 1 FROM groups WHERE slug = $1);
