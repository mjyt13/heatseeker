-- name: CreateSubject :one
INSERT INTO subjects (id, group_id, name, short_name, teacher, teacher_contact, color, semester, aliases, sort_order, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetSubject :one
SELECT * FROM subjects WHERE id = $1 AND group_id = $2;

-- name: ListSubjects :many
SELECT * FROM subjects
WHERE group_id = $1 AND (sqlc.arg(include_archived)::boolean OR archived_at IS NULL)
ORDER BY sort_order, name;

-- name: UpdateSubject :one
UPDATE subjects
SET name = $3, short_name = $4, teacher = $5, teacher_contact = $6, color = $7, semester = $8, aliases = $9, sort_order = $10
WHERE id = $1 AND group_id = $2
RETURNING *;

-- name: ArchiveSubject :exec
UPDATE subjects SET archived_at = now() WHERE id = $1 AND group_id = $2;

-- name: RestoreSubject :exec
UPDATE subjects SET archived_at = NULL WHERE id = $1 AND group_id = $2;

-- name: AddSubjectAliases :exec
UPDATE subjects
SET aliases = (SELECT array_agg(DISTINCT a) FROM unnest(aliases || sqlc.arg(new_aliases)::text[]) AS a)
WHERE id = $1 AND group_id = $2;
