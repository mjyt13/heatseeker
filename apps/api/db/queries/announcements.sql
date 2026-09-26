-- name: CreateAnnouncement :one
INSERT INTO announcements (id, group_id, author_id, title, body, urgent, pinned)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAnnouncement :one
SELECT * FROM announcements WHERE id = $1 AND deleted_at IS NULL;

-- name: ListAnnouncements :many
-- Pinned first, then newest; a page is small, so a limit is enough.
SELECT sqlc.embed(a), COALESCE(u.name, '')::text AS author_name
FROM announcements a
LEFT JOIN users u ON u.id = a.author_id
WHERE a.group_id = sqlc.arg(group_id)::uuid AND a.deleted_at IS NULL
ORDER BY a.pinned DESC, a.created_at DESC
LIMIT sqlc.arg(lim)::int;

-- name: UpdateAnnouncement :one
UPDATE announcements
SET title = sqlc.arg(title), body = sqlc.arg(body), urgent = sqlc.arg(urgent), pinned = sqlc.arg(pinned)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: DeleteAnnouncement :execrows
UPDATE announcements SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;
