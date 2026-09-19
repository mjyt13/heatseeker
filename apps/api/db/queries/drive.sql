-- name: UpsertDriveConnection :one
INSERT INTO drive_connections (id, group_id, root_folder_id, root_folder_name, drive_id, status, sync_interval_sec,
                               writable, created_by)
VALUES ($1, $2, $3, $4, $5, 'PENDING', $6, $7, $8)
ON CONFLICT (group_id) DO UPDATE
SET root_folder_id = EXCLUDED.root_folder_id,
    root_folder_name = EXCLUDED.root_folder_name,
    drive_id = EXCLUDED.drive_id,
    sync_interval_sec = EXCLUDED.sync_interval_sec,
    writable = EXCLUDED.writable,
    -- a different folder starts from scratch
    status = CASE WHEN drive_connections.root_folder_id = EXCLUDED.root_folder_id
                  THEN drive_connections.status ELSE 'PENDING' END,
    changes_page_token = CASE WHEN drive_connections.root_folder_id = EXCLUDED.root_folder_id
                              THEN drive_connections.changes_page_token ELSE NULL END,
    last_full_scan_at = CASE WHEN drive_connections.root_folder_id = EXCLUDED.root_folder_id
                             THEN drive_connections.last_full_scan_at ELSE NULL END,
    last_error = NULL
RETURNING *;

-- name: GetDriveConnection :one
SELECT * FROM drive_connections WHERE id = $1;

-- name: GetDriveConnectionByGroup :one
SELECT * FROM drive_connections WHERE group_id = $1;

-- name: DeleteDriveConnection :exec
DELETE FROM drive_connections WHERE id = $1;

-- name: ListDueDriveConnections :many
SELECT c.* FROM drive_connections c
JOIN groups g ON g.id = c.group_id AND g.archived_at IS NULL
WHERE c.status <> 'SYNCING'
  AND (c.last_sync_at IS NULL
       OR c.last_sync_at + make_interval(secs => c.sync_interval_sec) <= sqlc.arg(now)::timestamptz)
ORDER BY c.last_sync_at NULLS FIRST;

-- name: BeginDriveSync :execrows
UPDATE drive_connections
SET status = 'SYNCING', sync_started_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)
  AND (status <> 'SYNCING' OR sync_started_at IS NULL OR sync_started_at < sqlc.arg(stale_before)::timestamptz);

-- name: FinishDriveSync :exec
UPDATE drive_connections
SET status = sqlc.arg(status),
    last_error = sqlc.narg(last_error),
    last_error_code = sqlc.narg(last_error_code),
    sync_started_at = NULL,
    changes_page_token = COALESCE(sqlc.narg(page_token), changes_page_token),
    -- a failed run still counts as an attempt, so the scheduler backs off
    last_sync_at = sqlc.arg(completed_at)::timestamptz,
    last_full_scan_at = CASE WHEN sqlc.arg(full_scan)::boolean AND sqlc.arg(succeeded)::boolean
                             THEN sqlc.arg(completed_at)::timestamptz ELSE last_full_scan_at END
WHERE id = sqlc.arg(id);

-- name: GetDriveItem :one
SELECT * FROM drive_items WHERE connection_id = $1 AND drive_file_id = $2;

-- name: GetDriveItemByMaterial :one
SELECT * FROM drive_items WHERE material_id = $1 ORDER BY updated_at DESC LIMIT 1;

-- name: UpsertDriveItem :one
INSERT INTO drive_items (id, connection_id, drive_file_id, parent_id, path_cache, name, mime, is_folder, md5,
                         size_bytes, modified_time, web_view_link, material_id, state, classification, seen_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
ON CONFLICT (connection_id, drive_file_id) DO UPDATE
SET parent_id = EXCLUDED.parent_id,
    path_cache = EXCLUDED.path_cache,
    name = EXCLUDED.name,
    mime = EXCLUDED.mime,
    is_folder = EXCLUDED.is_folder,
    md5 = EXCLUDED.md5,
    size_bytes = EXCLUDED.size_bytes,
    modified_time = EXCLUDED.modified_time,
    web_view_link = EXCLUDED.web_view_link,
    -- the sync run's clock, compared against its start to find vanished files
    seen_at = EXCLUDED.seen_at
RETURNING *;

-- name: UpdateDriveItemState :exec
UPDATE drive_items
SET state = $2, material_id = $3, classification = $4, last_error = $5
WHERE id = $1;

-- name: ListDriveItems :many
SELECT * FROM drive_items
WHERE connection_id = $1 AND NOT is_folder
  AND (sqlc.narg(state)::text IS NULL OR state = sqlc.narg(state)::text)
ORDER BY path_cache, name
LIMIT $2 OFFSET $3;

-- name: ListDriveFolders :many
SELECT * FROM drive_items WHERE connection_id = $1 AND is_folder AND state <> 'DELETED';

-- name: ListUnseenDriveItems :many
SELECT * FROM drive_items WHERE connection_id = $1 AND seen_at < $2 AND state <> 'DELETED';

-- name: DeleteDriveItems :exec
DELETE FROM drive_items WHERE connection_id = $1;

-- name: DriveItemStats :one
SELECT
  count(*) FILTER (WHERE NOT is_folder AND state <> 'DELETED')::bigint AS files,
  count(*) FILTER (WHERE is_folder AND state <> 'DELETED')::bigint AS folders,
  count(*) FILTER (WHERE state IN ('LINKED', 'IMPORTED'))::bigint AS linked,
  count(*) FILTER (WHERE state = 'SKIPPED')::bigint AS skipped,
  count(*) FILTER (WHERE state = 'DELETED')::bigint AS deleted,
  count(*) FILTER (WHERE state = 'ERROR')::bigint AS errors
FROM drive_items WHERE connection_id = $1;

-- name: ListReclassifyCandidates :many
-- Drive files waiting in the Inbox only because the classifier was unsure:
-- new subjects or aliases may resolve them. Manual decisions are left alone.
SELECT i.* FROM drive_items i
JOIN drive_connections c ON c.id = i.connection_id
JOIN materials m ON m.id = i.material_id AND m.group_id = c.group_id
WHERE c.group_id = $1
  AND NOT i.is_folder
  AND i.state = 'LINKED'
  AND m.source = 'GDRIVE'
  AND m.status = 'ACTIVE'
  AND m.needs_review
  AND m.review_reason = 'LOW_CONFIDENCE'
  AND coalesce(m.classification ->> 'method', 'auto') = 'auto'
ORDER BY i.path_cache, i.name;

-- name: UpsertDrivePublisher :one
INSERT INTO drive_publishers (group_id, google_email, refresh_token_enc, scopes, connected_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (group_id) DO UPDATE
SET google_email = EXCLUDED.google_email, refresh_token_enc = EXCLUDED.refresh_token_enc,
    scopes = EXCLUDED.scopes, connected_by = EXCLUDED.connected_by, last_error = NULL,
    created_at = now()
RETURNING *;

-- name: GetDrivePublisher :one
SELECT * FROM drive_publishers WHERE group_id = $1;

-- name: DeleteDrivePublisher :exec
DELETE FROM drive_publishers WHERE group_id = $1;

-- name: SetDrivePublisherError :exec
UPDATE drive_publishers SET last_error = $2 WHERE group_id = $1;
