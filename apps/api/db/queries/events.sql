-- name: InsertGroupEvent :one
INSERT INTO group_events (id, group_id, seq, kind, actor_id, entity_type, entity_id, payload, audit)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListGroupEventsSince :many
SELECT * FROM group_events
WHERE group_id = $1 AND seq > $2
ORDER BY seq
LIMIT $3;

-- name: ListGroupAuditEvents :many
SELECT * FROM group_events
WHERE group_id = $1 AND audit = true AND (sqlc.narg(before_seq)::bigint IS NULL OR seq < sqlc.narg(before_seq)::bigint)
ORDER BY seq DESC
LIMIT $2;

-- name: OldestGroupEventSeq :one
SELECT COALESCE(min(seq), 0)::bigint FROM group_events WHERE group_id = $1;

-- name: DeleteGroupEventsBefore :execrows
DELETE FROM group_events WHERE created_at < $1;
