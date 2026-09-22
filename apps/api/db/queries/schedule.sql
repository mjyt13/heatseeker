-- name: CreateScheduleEvent :one
INSERT INTO schedule_events (id, group_id, subject_id, client_id, title, kind, starts_at, ends_at, timezone,
                             location, teacher, note, rrule, rrule_until, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $15)
RETURNING *;

-- name: GetScheduleEvent :one
SELECT * FROM schedule_events WHERE id = $1 AND deleted_at IS NULL;

-- name: GetScheduleEventByClientID :one
SELECT * FROM schedule_events WHERE group_id = $1 AND client_id = $2 AND deleted_at IS NULL;

-- name: ListScheduleEventsInWindow :many
-- Events that may have a class overlapping [window_from, window_to): one-off
-- classes inside it, series running at that time, and anything with a class
-- moved into it.
SELECT e.* FROM schedule_events e
WHERE e.group_id = sqlc.arg(group_id) AND e.deleted_at IS NULL
  AND ((e.starts_at < sqlc.arg(window_to)::timestamptz
        AND ((e.rrule IS NULL AND e.ends_at > sqlc.arg(window_from)::timestamptz)
             OR (e.rrule IS NOT NULL
                 AND (e.rrule_until IS NULL OR e.rrule_until >= (sqlc.arg(window_from)::timestamptz - interval '2 days')::date))))
       OR EXISTS (SELECT 1 FROM schedule_exceptions x
                  WHERE x.event_id = e.id
                    AND x.starts_at < sqlc.arg(window_to)::timestamptz
                    AND x.ends_at > sqlc.arg(window_from)::timestamptz))
ORDER BY e.starts_at, e.id;

-- name: ListGroupScheduleEvents :many
SELECT * FROM schedule_events WHERE group_id = $1 AND deleted_at IS NULL ORDER BY starts_at, id;

-- name: UpdateScheduleEvent :one
-- Only when nobody changed the event since the editor loaded it.
UPDATE schedule_events
SET subject_id = $3, title = $4, kind = $5, starts_at = $6, ends_at = $7, timezone = $8,
    location = $9, teacher = $10, note = $11, rrule = $12, rrule_until = $13, updated_by = $14,
    version = version + 1
WHERE id = $1 AND version = $2 AND deleted_at IS NULL
RETURNING *;

-- name: EndScheduleSeries :one
UPDATE schedule_events
SET rrule_until = sqlc.arg(until)::date, updated_by = sqlc.arg(updated_by), version = version + 1
WHERE id = sqlc.arg(id) AND version = sqlc.arg(version) AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteScheduleEvent :execrows
UPDATE schedule_events
SET deleted_at = sqlc.arg(deleted_at), updated_by = sqlc.arg(updated_by), version = version + 1
WHERE id = sqlc.arg(id) AND version = sqlc.arg(version) AND deleted_at IS NULL;

-- name: ListScheduleExceptions :many
SELECT * FROM schedule_exceptions WHERE event_id = ANY (sqlc.arg(event_ids)::uuid[]) ORDER BY event_id, original_date;

-- name: GetScheduleException :one
SELECT * FROM schedule_exceptions WHERE event_id = $1 AND original_date = $2;

-- name: UpsertScheduleException :one
INSERT INTO schedule_exceptions (event_id, original_date, kind, starts_at, ends_at, location, teacher, note, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (event_id, original_date) DO UPDATE
SET kind = EXCLUDED.kind, starts_at = EXCLUDED.starts_at, ends_at = EXCLUDED.ends_at,
    location = EXCLUDED.location, teacher = EXCLUDED.teacher, note = EXCLUDED.note, updated_by = EXCLUDED.updated_by
RETURNING *;

-- name: DeleteScheduleException :execrows
DELETE FROM schedule_exceptions WHERE event_id = $1 AND original_date = $2;

-- name: MoveScheduleExceptions :exec
-- A series split in two: changes from the split date on follow the new part.
UPDATE schedule_exceptions SET event_id = sqlc.arg(to_event_id)
WHERE event_id = sqlc.arg(from_event_id) AND original_date >= sqlc.arg(from_date)::date;

-- name: DeleteScheduleExceptionsFrom :exec
DELETE FROM schedule_exceptions WHERE event_id = sqlc.arg(event_id) AND original_date >= sqlc.arg(from_date)::date;
