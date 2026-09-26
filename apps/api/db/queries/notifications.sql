-- name: GetNotifierCursor :one
SELECT last_seq FROM notifier_cursors WHERE group_id = $1;

-- name: SetNotifierCursor :exec
INSERT INTO notifier_cursors (group_id, last_seq, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (group_id) DO UPDATE SET last_seq = GREATEST(notifier_cursors.last_seq, EXCLUDED.last_seq), updated_at = now();

-- name: ListGroupsWithPendingEvents :many
-- Groups whose log has moved past the reader. A new group starts at its
-- current head: nobody is notified about what happened before they joined.
SELECT g.id
FROM groups g
LEFT JOIN notifier_cursors c ON c.group_id = g.id
WHERE g.archived_at IS NULL AND g.last_seq > COALESCE(c.last_seq, 0)
ORDER BY g.id
LIMIT $1;

-- name: ListNotifyRecipients :many
-- Active members of a group who want this type of notification: not the
-- actor, not switched off, not muted, and — for a discussion — not people who
-- have already read the message on another screen.
SELECT m.user_id, COALESCE(u.timezone, 'UTC')::text AS timezone
FROM memberships m
JOIN users u ON u.id = m.user_id
LEFT JOIN notification_prefs p
  ON p.user_id = m.user_id AND p.group_id = m.group_id AND p.type = sqlc.arg(type)::text
WHERE m.group_id = sqlc.arg(group_id)::uuid
  AND m.status = 'ACTIVE'
  AND m.user_id <> ALL (sqlc.arg(exclude)::uuid[])
  AND COALESCE(p.enabled, sqlc.arg(default_enabled)::boolean)
  AND NOT EXISTS (
    SELECT 1 FROM notification_mutes mu
    WHERE mu.user_id = m.user_id
      AND mu.group_id = m.group_id
      AND mu.until > sqlc.arg(now)::timestamptz
      AND ((mu.scope_type = 'GROUP' AND mu.scope_id = m.group_id::text)
        OR (mu.scope_type = 'TYPE' AND mu.scope_id = sqlc.arg(type)::text)
        OR (mu.scope_type = 'SUBJECT' AND mu.scope_id = sqlc.narg(subject_id)::text)
        OR (mu.scope_type = 'THREAD' AND mu.scope_id = sqlc.narg(thread_id)::text))
  )
  AND (sqlc.narg(thread_id)::text IS NULL OR NOT EXISTS (
    SELECT 1 FROM thread_reads r
    WHERE r.thread_id = sqlc.narg(thread_id)::uuid
      AND r.user_id = m.user_id
      AND r.last_read_seq >= sqlc.arg(seq)::bigint
  ))
ORDER BY m.user_id;

-- name: ListThreadParticipants :many
-- Everybody who has written in a discussion — the people it belongs to when
-- the whole group is not meant to hear about it (open question №7).
SELECT DISTINCT author_id AS user_id FROM messages
WHERE thread_id = $1 AND author_id IS NOT NULL AND deleted_at IS NULL;

-- name: GetNotifyMessage :one
-- The text a push carries, with its author and discussion.
SELECT m.id, m.thread_id, m.seq, m.author_id, m.reply_to_id, m.deleted_at, m.hidden_for_all_at,
       left(m.body, 300)::text AS body,
       COALESCE(u.name, '')::text AS author_name,
       t.target_type, t.target_id, t.subject_id, t.title,
       COALESCE(s.name, '')::text AS subject_name,
       r.author_id AS reply_author_id
FROM messages m
JOIN threads t ON t.id = m.thread_id
LEFT JOIN users u ON u.id = m.author_id
LEFT JOIN subjects s ON s.id = t.subject_id
LEFT JOIN messages r ON r.id = m.reply_to_id
WHERE m.id = $1;

-- name: GetNotifyUserName :one
SELECT COALESCE(name, '')::text FROM users WHERE id = $1;

-- name: GetNotifySubjectName :one
SELECT COALESCE(name, '')::text FROM subjects WHERE id = $1;

-- name: GetNotifyMaterialOwner :one
SELECT uploader_id FROM materials WHERE id = $1;

-- name: GetNotifyTask :one
SELECT id, group_id, subject_id, created_by, title, assign_mode, visibility, due_at FROM tasks WHERE id = $1;

-- name: ListTaskAssignees :many
-- Everybody with their own part of a task, done or not.
SELECT a.user_id, a.status FROM task_assignments a WHERE a.task_id = $1;

-- name: InsertNotification :many
-- Returns nothing when the same fact was already delivered to this user.
INSERT INTO notifications (id, user_id, group_id, type, title, body, data, seq, dedupe_key)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (user_id, dedupe_key) DO NOTHING
RETURNING *;

-- name: ListNotifications :many
SELECT * FROM notifications
WHERE user_id = sqlc.arg(user_id)::uuid
  AND (sqlc.narg(group_id)::uuid IS NULL OR group_id = sqlc.narg(group_id)::uuid)
  AND (NOT sqlc.arg(unread_only)::boolean OR read_at IS NULL)
  AND (sqlc.narg(before)::timestamptz IS NULL
       OR created_at < sqlc.narg(before)::timestamptz
       OR (created_at = sqlc.narg(before)::timestamptz AND id < sqlc.narg(before_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(lim)::int;

-- name: CountUnreadNotifications :one
SELECT count(*)::int FROM notifications
WHERE user_id = sqlc.arg(user_id)::uuid
  AND read_at IS NULL
  AND (sqlc.narg(group_id)::uuid IS NULL OR group_id = sqlc.narg(group_id)::uuid);

-- name: MarkNotificationsRead :execrows
UPDATE notifications SET read_at = now()
WHERE user_id = sqlc.arg(user_id)::uuid
  AND read_at IS NULL
  AND (sqlc.narg(group_id)::uuid IS NULL OR group_id = sqlc.narg(group_id)::uuid)
  AND (sqlc.arg(all_of_them)::boolean OR id = ANY (sqlc.arg(ids)::uuid[]));

-- name: ListNotificationPrefs :many
SELECT * FROM notification_prefs WHERE user_id = $1 AND group_id = $2;

-- name: SetNotificationPref :exec
INSERT INTO notification_prefs (user_id, group_id, type, enabled)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, group_id, type) DO UPDATE SET enabled = EXCLUDED.enabled;

-- name: GetNotificationSettings :one
SELECT * FROM notification_settings WHERE user_id = $1;

-- name: SaveNotificationSettings :one
INSERT INTO notification_settings (user_id, push_enabled, quiet_from, quiet_to, urgent_in_quiet)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id) DO UPDATE
SET push_enabled = EXCLUDED.push_enabled, quiet_from = EXCLUDED.quiet_from,
    quiet_to = EXCLUDED.quiet_to, urgent_in_quiet = EXCLUDED.urgent_in_quiet
RETURNING *;

-- name: ListNotificationMutes :many
SELECT * FROM notification_mutes
WHERE user_id = sqlc.arg(user_id)::uuid
  AND until > now()
  AND (sqlc.narg(group_id)::uuid IS NULL OR group_id = sqlc.narg(group_id)::uuid)
ORDER BY until;

-- name: SetNotificationMute :one
INSERT INTO notification_mutes (id, user_id, group_id, scope_type, scope_id, until)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, group_id, scope_type, scope_id) DO UPDATE SET until = EXCLUDED.until
RETURNING *;

-- name: DeleteNotificationMute :execrows
DELETE FROM notification_mutes
WHERE user_id = sqlc.arg(user_id)::uuid AND group_id = sqlc.arg(group_id)::uuid
  AND scope_type = sqlc.arg(scope_type)::text AND scope_id = sqlc.arg(scope_id)::text;

-- name: DeleteExpiredNotificationMutes :execrows
DELETE FROM notification_mutes WHERE until <= $1;

-- name: DeleteOldNotifications :execrows
DELETE FROM notifications WHERE created_at < $1;

-- name: ListPushTargets :many
-- Devices to push to, with the owner's quiet hours and timezone.
SELECT d.id AS device_id, d.user_id, d.platform, d.push_provider, d.push_token,
       COALESCE(u.timezone, 'UTC')::text AS timezone,
       COALESCE(s.push_enabled, true)::boolean AS push_enabled,
       s.quiet_from, s.quiet_to,
       COALESCE(s.urgent_in_quiet, false)::boolean AS urgent_in_quiet
FROM devices d
JOIN users u ON u.id = d.user_id
LEFT JOIN notification_settings s ON s.user_id = d.user_id
WHERE d.user_id = ANY (sqlc.arg(user_ids)::uuid[])
  AND d.enabled
  AND d.push_provider = 'EXPO'
  AND d.push_token IS NOT NULL AND d.push_token <> '';

-- name: ListNotificationsForPush :many
SELECT * FROM notifications WHERE id = ANY (sqlc.arg(ids)::uuid[]) ORDER BY created_at;

-- name: InsertNotificationDelivery :exec
INSERT INTO notification_deliveries (id, notification_id, channel, device_id, status, error, sent_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);
