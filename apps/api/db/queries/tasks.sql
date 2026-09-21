-- name: CreateTask :one
INSERT INTO tasks (id, group_id, subject_id, created_by, client_id, title, description, kind, status,
                   priority, assign_mode, visibility, due_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: GetTask :one
SELECT * FROM tasks WHERE id = $1 AND deleted_at IS NULL;

-- name: GetTaskByClientID :one
SELECT * FROM tasks WHERE group_id = $1 AND client_id = $2 AND deleted_at IS NULL;

-- name: GetTaskView :one
SELECT sqlc.embed(t),
       COALESCE((SELECT a.status FROM task_assignments a WHERE a.task_id = t.id AND a.user_id = sqlc.arg(actor_id)), '')::text AS my_status,
       COALESCE((SELECT array_agg(a.user_id ORDER BY a.created_at) FROM task_assignments a WHERE a.task_id = t.id), '{}')::uuid[] AS assignee_ids,
       COALESCE((SELECT array_agg(att.material_id ORDER BY att.created_at) FROM task_attachments att WHERE att.task_id = t.id), '{}')::uuid[] AS attachment_ids,
       (SELECT count(*) FROM task_assignments a WHERE a.task_id = t.id AND a.status = 'DONE')::int AS done_count,
       (SELECT count(*) FROM task_assignments a WHERE a.task_id = t.id)::int AS assigned_count
FROM tasks t
WHERE t.id = sqlc.arg(id) AND t.deleted_at IS NULL;

-- name: ListTasks :many
SELECT sqlc.embed(t),
       COALESCE((SELECT a.status FROM task_assignments a WHERE a.task_id = t.id AND a.user_id = sqlc.arg(actor_id)), '')::text AS my_status,
       COALESCE((SELECT array_agg(a.user_id ORDER BY a.created_at) FROM task_assignments a WHERE a.task_id = t.id), '{}')::uuid[] AS assignee_ids,
       COALESCE((SELECT array_agg(att.material_id ORDER BY att.created_at) FROM task_attachments att WHERE att.task_id = t.id), '{}')::uuid[] AS attachment_ids,
       (SELECT count(*) FROM task_assignments a WHERE a.task_id = t.id AND a.status = 'DONE')::int AS done_count,
       (SELECT count(*) FROM task_assignments a WHERE a.task_id = t.id)::int AS assigned_count
FROM tasks t
WHERE t.group_id = sqlc.arg(group_id)
  AND t.deleted_at IS NULL
  -- Private tasks belong to their author only.
  AND (t.visibility = 'GROUP' OR t.created_by = sqlc.arg(actor_id))
  AND (sqlc.narg(subject_id)::uuid IS NULL OR t.subject_id = sqlc.narg(subject_id)::uuid)
  AND (NOT sqlc.arg(no_subject)::boolean OR t.subject_id IS NULL)
  AND (sqlc.narg(kind)::text IS NULL OR t.kind = sqlc.narg(kind)::text)
  AND (cardinality(sqlc.arg(statuses)::text[]) = 0 OR t.status = ANY (sqlc.arg(statuses)::text[]))
  AND (NOT sqlc.arg(open_only)::boolean OR t.status NOT IN ('DONE', 'CANCELLED'))
  AND (NOT sqlc.arg(overdue)::boolean
       OR (t.status NOT IN ('DONE', 'CANCELLED') AND t.due_at IS NOT NULL AND t.due_at < sqlc.arg(now)::timestamptz))
  AND (sqlc.narg(due_before)::timestamptz IS NULL OR (t.due_at IS NOT NULL AND t.due_at <= sqlc.narg(due_before)::timestamptz))
  -- "Mine": created by me, handed to me, or handed to everybody.
  AND (NOT sqlc.arg(mine)::boolean
       OR t.created_by = sqlc.arg(actor_id)
       OR t.assign_mode = 'ALL'
       OR EXISTS (SELECT 1 FROM task_assignments a WHERE a.task_id = t.id AND a.user_id = sqlc.arg(actor_id)))
  AND (sqlc.narg(like_pattern)::text IS NULL
       OR t.title ILIKE '%' || sqlc.narg(like_pattern)::text || '%' ESCAPE '\'
       OR t.description ILIKE '%' || sqlc.narg(like_pattern)::text || '%' ESCAPE '\')
ORDER BY (t.pinned_at IS NULL), t.pinned_at DESC, (t.due_at IS NULL), t.due_at, t.created_at DESC, t.id
LIMIT sqlc.arg(max_rows);

-- name: CountTasks :one
SELECT
  count(*) FILTER (WHERE t.status = 'TODO')::int                    AS todo_count,
  count(*) FILTER (WHERE t.status = 'IN_PROGRESS')::int             AS in_progress_count,
  count(*) FILTER (WHERE t.status = 'IN_REVIEW')::int               AS in_review_count,
  count(*) FILTER (WHERE t.status = 'DONE')::int                    AS done_count,
  count(*) FILTER (WHERE t.status = 'CANCELLED')::int               AS cancelled_count,
  count(*) FILTER (WHERE t.status NOT IN ('DONE', 'CANCELLED'))::int AS open_count,
  count(*) FILTER (WHERE t.status NOT IN ('DONE', 'CANCELLED')
                     AND t.due_at IS NOT NULL AND t.due_at < sqlc.arg(now)::timestamptz)::int AS overdue_count,
  count(*) FILTER (WHERE t.status NOT IN ('DONE', 'CANCELLED')
                     AND t.due_at IS NOT NULL AND t.due_at >= sqlc.arg(now)::timestamptz
                     AND t.due_at <= sqlc.arg(due_soon_before)::timestamptz)::int AS due_soon_count,
  count(*) FILTER (WHERE t.status NOT IN ('DONE', 'CANCELLED')
                     AND (t.created_by = sqlc.arg(actor_id) OR t.assign_mode = 'ALL'
                          OR EXISTS (SELECT 1 FROM task_assignments a WHERE a.task_id = t.id AND a.user_id = sqlc.arg(actor_id)))
                     AND NOT EXISTS (SELECT 1 FROM task_assignments a WHERE a.task_id = t.id
                                       AND a.user_id = sqlc.arg(actor_id) AND a.status IN ('DONE', 'CANCELLED')))::int AS mine_count
FROM tasks t
WHERE t.group_id = sqlc.arg(group_id)
  AND t.deleted_at IS NULL
  AND (t.visibility = 'GROUP' OR t.created_by = sqlc.arg(actor_id));

-- name: UpdateTask :one
UPDATE tasks
SET subject_id = sqlc.narg(subject_id)::uuid,
    title = sqlc.arg(title),
    description = sqlc.arg(description),
    kind = sqlc.arg(kind),
    priority = sqlc.arg(priority),
    assign_mode = sqlc.arg(assign_mode),
    visibility = sqlc.arg(visibility),
    due_at = sqlc.narg(due_at)::timestamptz,
    notified_offsets = CASE WHEN sqlc.arg(reset_notified)::boolean THEN '{}'::integer[] ELSE notified_offsets END
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: SetTaskStatus :one
UPDATE tasks
SET status = sqlc.arg(status), completed_at = sqlc.narg(completed_at)::timestamptz
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: SetTaskPinned :one
UPDATE tasks
SET pinned_by = sqlc.narg(pinned_by)::uuid, pinned_at = sqlc.narg(pinned_at)::timestamptz
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteTask :exec
UPDATE tasks SET deleted_at = sqlc.arg(deleted_at)::timestamptz WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListTaskAssignments :many
SELECT * FROM task_assignments WHERE task_id = $1 ORDER BY created_at;

-- name: DeleteTaskAssignments :exec
DELETE FROM task_assignments
WHERE task_id = sqlc.arg(task_id) AND NOT (user_id = ANY (sqlc.arg(user_ids)::uuid[]));

-- name: InsertTaskAssignments :exec
INSERT INTO task_assignments (task_id, user_id)
SELECT sqlc.arg(task_id)::uuid, unnest(sqlc.arg(user_ids)::uuid[])
ON CONFLICT DO NOTHING;

-- name: UpsertTaskAssignmentStatus :one
INSERT INTO task_assignments (task_id, user_id, status, completed_at)
VALUES (sqlc.arg(task_id), sqlc.arg(user_id), sqlc.arg(status), sqlc.narg(completed_at)::timestamptz)
ON CONFLICT (task_id, user_id) DO UPDATE
SET status = EXCLUDED.status, completed_at = EXCLUDED.completed_at
RETURNING *;

-- name: DeleteTaskAttachments :exec
DELETE FROM task_attachments
WHERE task_id = sqlc.arg(task_id) AND NOT (material_id = ANY (sqlc.arg(material_ids)::uuid[]));

-- name: InsertTaskAttachments :exec
INSERT INTO task_attachments (task_id, material_id)
SELECT sqlc.arg(task_id)::uuid, m.id
FROM materials m
WHERE m.id = ANY (sqlc.arg(material_ids)::uuid[])
  AND m.group_id = sqlc.arg(group_id)
  AND m.status <> 'DELETED'
  -- A file kept in another task stays there (D43).
  AND (m.task_id IS NULL OR m.task_id = sqlc.arg(task_id)::uuid)
ON CONFLICT DO NOTHING;

-- name: ListTaskDueReminders :many
-- Tasks whose deadline reminder is due: one row per offset that fired.
SELECT sqlc.embed(t), o.offset_minutes::int AS offset_minutes
FROM tasks t
JOIN unnest(sqlc.arg(offsets)::int[]) AS o (offset_minutes) ON true
WHERE t.deleted_at IS NULL
  AND t.status NOT IN ('DONE', 'CANCELLED')
  AND t.due_at IS NOT NULL
  AND t.due_at - make_interval(mins => o.offset_minutes) <= sqlc.arg(now)::timestamptz
  -- Deadlines long past are not announced (a task imported after the fact).
  AND t.due_at >= sqlc.arg(now)::timestamptz - make_interval(mins => sqlc.arg(grace_minutes)::int)
  AND NOT (o.offset_minutes = ANY (t.notified_offsets))
ORDER BY t.due_at, o.offset_minutes
LIMIT sqlc.arg(max_rows);

-- name: MarkTaskNotified :exec
UPDATE tasks
SET notified_offsets = (SELECT array_agg(DISTINCT x) FROM unnest(notified_offsets || sqlc.arg(offsets)::integer[]) AS x)
WHERE id = sqlc.arg(id);

-- name: AddTaskAttachment :exec
INSERT INTO task_attachments (task_id, material_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;
