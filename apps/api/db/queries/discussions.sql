-- name: GetThreadByTarget :one
SELECT * FROM threads WHERE group_id = $1 AND target_type = $2 AND target_id = $3;

-- name: GetThread :one
SELECT * FROM threads WHERE id = $1;

-- name: EnsureThread :one
-- The no-op update makes RETURNING yield the existing row on conflict.
INSERT INTO threads (id, group_id, target_type, target_id, subject_id, title, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (group_id, target_type, target_id) DO UPDATE SET target_id = EXCLUDED.target_id
RETURNING *;

-- name: ListThreadSummaries :many
SELECT sqlc.embed(t),
       (SELECT count(*) FROM messages m
        WHERE m.thread_id = t.id
          AND m.seq > COALESCE(r.last_read_seq, 0)
          AND m.deleted_at IS NULL
          AND m.hidden_for_all_at IS NULL
          AND m.author_id IS DISTINCT FROM sqlc.arg(viewer_id)::uuid
          AND NOT EXISTS (SELECT 1 FROM message_hides h WHERE h.message_id = m.id AND h.user_id = sqlc.arg(viewer_id)::uuid)
       )::int AS unread,
       lm.author_id AS last_author_id,
       COALESCE(lu.name, '')::text AS last_author_name,
       (lm.created_at IS NOT NULL)::boolean AS has_last,
       COALESCE(lm.body, '')::text AS last_body,
       COALESCE(lm.created_at, t.created_at)::timestamptz AS last_created_at
FROM threads t
LEFT JOIN thread_reads r ON r.thread_id = t.id AND r.user_id = sqlc.arg(viewer_id)::uuid
LEFT JOIN LATERAL (
  SELECT m.author_id, left(m.body, 200) AS body, m.created_at
  FROM messages m
  WHERE m.thread_id = t.id AND m.deleted_at IS NULL AND m.hidden_for_all_at IS NULL
  ORDER BY m.seq DESC
  LIMIT 1
) lm ON true
LEFT JOIN users lu ON lu.id = lm.author_id
WHERE t.group_id = sqlc.arg(group_id)::uuid AND t.message_count > 0
ORDER BY t.last_message_at DESC NULLS LAST, t.id;

-- name: TouchThread :exec
UPDATE threads
SET message_count = message_count + 1,
    last_seq = GREATEST(last_seq, sqlc.arg(seq)::bigint),
    last_message_at = sqlc.arg(at)::timestamptz
WHERE id = sqlc.arg(id);

-- name: AdjustThreadMessageCount :exec
UPDATE threads SET message_count = GREATEST(message_count + sqlc.arg(delta)::int, 0) WHERE id = sqlc.arg(id);

-- name: CreateMessage :one
INSERT INTO messages (id, group_id, thread_id, author_id, client_id, seq, body, reply_to_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetMessage :one
SELECT * FROM messages WHERE id = $1;

-- name: GetMessageByClientID :one
SELECT * FROM messages WHERE thread_id = $1 AND client_id = $2;

-- name: GetMessageView :one
SELECT sqlc.embed(m),
       COALESCE(u.name, '')::text AS author_name,
       EXISTS (SELECT 1 FROM message_hides h WHERE h.message_id = m.id AND h.user_id = sqlc.arg(viewer_id)::uuid) AS hidden_by_me,
       rm.id AS reply_id,
       rm.author_id AS reply_author_id,
       COALESCE(ru.name, '')::text AS reply_author_name,
       COALESCE(left(rm.body, 200), '')::text AS reply_body,
       (rm.deleted_at IS NOT NULL)::boolean AS reply_deleted,
       (rm.hidden_for_all_at IS NOT NULL)::boolean AS reply_hidden_for_all
FROM messages m
LEFT JOIN users u ON u.id = m.author_id
LEFT JOIN messages rm ON rm.id = m.reply_to_id
LEFT JOIN users ru ON ru.id = rm.author_id
WHERE m.id = sqlc.arg(id)::uuid;

-- name: ListMessagesBefore :many
-- The newest page (before_seq null) or older messages, newest first.
SELECT sqlc.embed(m),
       COALESCE(u.name, '')::text AS author_name,
       EXISTS (SELECT 1 FROM message_hides h WHERE h.message_id = m.id AND h.user_id = sqlc.arg(viewer_id)::uuid) AS hidden_by_me,
       rm.id AS reply_id,
       rm.author_id AS reply_author_id,
       COALESCE(ru.name, '')::text AS reply_author_name,
       COALESCE(left(rm.body, 200), '')::text AS reply_body,
       (rm.deleted_at IS NOT NULL)::boolean AS reply_deleted,
       (rm.hidden_for_all_at IS NOT NULL)::boolean AS reply_hidden_for_all
FROM messages m
LEFT JOIN users u ON u.id = m.author_id
LEFT JOIN messages rm ON rm.id = m.reply_to_id
LEFT JOIN users ru ON ru.id = rm.author_id
WHERE m.thread_id = sqlc.arg(thread_id)::uuid
  AND (sqlc.narg(before_seq)::bigint IS NULL OR m.seq < sqlc.narg(before_seq)::bigint)
ORDER BY m.seq DESC
LIMIT sqlc.arg(max_rows);

-- name: ListMessagesAfter :many
-- Messages newer than after_seq, oldest first (catching up a thread).
SELECT sqlc.embed(m),
       COALESCE(u.name, '')::text AS author_name,
       EXISTS (SELECT 1 FROM message_hides h WHERE h.message_id = m.id AND h.user_id = sqlc.arg(viewer_id)::uuid) AS hidden_by_me,
       rm.id AS reply_id,
       rm.author_id AS reply_author_id,
       COALESCE(ru.name, '')::text AS reply_author_name,
       COALESCE(left(rm.body, 200), '')::text AS reply_body,
       (rm.deleted_at IS NOT NULL)::boolean AS reply_deleted,
       (rm.hidden_for_all_at IS NOT NULL)::boolean AS reply_hidden_for_all
FROM messages m
LEFT JOIN users u ON u.id = m.author_id
LEFT JOIN messages rm ON rm.id = m.reply_to_id
LEFT JOIN users ru ON ru.id = rm.author_id
WHERE m.thread_id = sqlc.arg(thread_id)::uuid AND m.seq > sqlc.arg(after_seq)::bigint
ORDER BY m.seq
LIMIT sqlc.arg(max_rows);

-- name: EditMessage :one
UPDATE messages SET body = $2, edited_at = $3 WHERE id = $1 RETURNING *;

-- name: SoftDeleteMessage :one
UPDATE messages SET deleted_at = $2 WHERE id = $1 RETURNING *;

-- name: UndeleteMessage :one
UPDATE messages SET deleted_at = NULL WHERE id = $1 RETURNING *;

-- name: SetMessageHiddenForAll :one
UPDATE messages SET hidden_for_all_by = $2, hidden_for_all_at = $3 WHERE id = $1 RETURNING *;

-- name: HideMessage :exec
INSERT INTO message_hides (message_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: UnhideMessage :exec
DELETE FROM message_hides WHERE message_id = $1 AND user_id = $2;

-- name: ListHiddenByUser :many
SELECT sqlc.embed(m),
       sqlc.embed(t),
       COALESCE(u.name, '')::text AS author_name
FROM message_hides h
JOIN messages m ON m.id = h.message_id
JOIN threads t ON t.id = m.thread_id
LEFT JOIN users u ON u.id = m.author_id
WHERE h.user_id = sqlc.arg(user_id)::uuid AND m.group_id = sqlc.arg(group_id)::uuid AND m.deleted_at IS NULL
ORDER BY h.created_at DESC, m.id
LIMIT sqlc.arg(max_rows);

-- name: MarkThreadRead :exec
INSERT INTO thread_reads (thread_id, user_id, last_read_seq, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (thread_id, user_id) DO UPDATE
SET last_read_seq = GREATEST(thread_reads.last_read_seq, EXCLUDED.last_read_seq), updated_at = now();

-- name: GetThreadReadSeq :one
SELECT COALESCE((SELECT last_read_seq FROM thread_reads WHERE thread_id = $1 AND user_id = $2), 0)::bigint;
