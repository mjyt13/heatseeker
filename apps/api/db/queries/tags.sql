-- name: CreateTag :one
INSERT INTO tags (id, group_id, name, slug, color, kind, subject_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetTag :one
SELECT * FROM tags WHERE id = $1 AND group_id = $2;

-- name: GetTagBySlug :one
SELECT * FROM tags WHERE group_id = $1 AND slug = $2;

-- name: GetTagBySubject :one
SELECT * FROM tags WHERE subject_id = $1;

-- name: ListTags :many
SELECT * FROM tags WHERE group_id = $1 ORDER BY kind, name;

-- name: UpdateTag :one
UPDATE tags SET name = $3, slug = $4, color = $5 WHERE id = $1 AND group_id = $2 RETURNING *;

-- name: DeleteTag :exec
DELETE FROM tags WHERE id = $1 AND group_id = $2 AND kind <> 'SUBJECT';
