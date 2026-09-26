-- name: CreateReminder :one
INSERT INTO reminders (id, group_id, user_id, title, note, remind_at, repeat, target_type, target_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetReminder :one
SELECT * FROM reminders WHERE id = $1;

-- name: ListReminders :many
-- One person's own reminders: open ones first, then what is already done.
SELECT * FROM reminders
WHERE user_id = sqlc.arg(user_id)::uuid
  AND group_id = sqlc.arg(group_id)::uuid
  AND (NOT sqlc.arg(open_only)::boolean OR status <> 'DONE')
ORDER BY (status = 'DONE'), remind_at
LIMIT sqlc.arg(lim)::int;

-- name: UpdateReminder :one
UPDATE reminders
SET title = sqlc.arg(title), note = sqlc.arg(note), remind_at = sqlc.arg(remind_at),
    repeat = sqlc.arg(repeat), target_type = sqlc.narg(target_type), target_id = sqlc.narg(target_id),
    status = sqlc.arg(status), last_fired_at = sqlc.narg(last_fired_at)
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id)::uuid
RETURNING *;

-- name: DeleteReminder :execrows
DELETE FROM reminders WHERE id = $1 AND user_id = $2;

-- name: ListDueReminders :many
-- What the scanner owes people right now.
SELECT * FROM reminders
WHERE status = 'SCHEDULED' AND remind_at <= sqlc.arg(now)::timestamptz
ORDER BY remind_at
LIMIT sqlc.arg(lim)::int;
