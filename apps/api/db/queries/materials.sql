-- name: CreateMaterial :one
INSERT INTO materials (id, group_id, subject_id, uploader_id, title, description, kind, source, status,
                       classification, needs_review, review_reason, sort_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id, group_id, subject_id, uploader_id, title, description, kind, source, status, current_version_id,
          classification, needs_review, review_reason, download_count, sort_at, archived_by, archived_at,
          deleted_by, deleted_at, created_at, updated_at;

-- name: GetMaterial :one
SELECT id, group_id, subject_id, uploader_id, title, description, kind, source, status, current_version_id,
       classification, needs_review, review_reason, download_count, sort_at, archived_by, archived_at,
       deleted_by, deleted_at, created_at, updated_at
FROM materials WHERE id = $1;

-- name: GetMaterialView :one
SELECT m.id, m.group_id, m.subject_id, m.uploader_id, m.title, m.description, m.kind, m.source, m.status,
       m.current_version_id, m.classification, m.needs_review, m.review_reason, m.download_count, m.sort_at,
       m.archived_by, m.archived_at, m.deleted_by, m.deleted_at, m.created_at, m.updated_at,
       sqlc.embed(v)
FROM materials m
JOIN material_versions v ON v.id = m.current_version_id
WHERE m.id = $1;

-- name: ListMaterials :many
SELECT m.id, m.group_id, m.subject_id, m.uploader_id, m.title, m.description, m.kind, m.source, m.status,
       m.current_version_id, m.classification, m.needs_review, m.review_reason, m.download_count, m.sort_at,
       m.archived_by, m.archived_at, m.deleted_by, m.deleted_at, m.created_at, m.updated_at,
       sqlc.embed(v)
FROM materials m
JOIN material_versions v ON v.id = m.current_version_id
WHERE m.group_id = sqlc.arg(group_id)
  AND m.status = sqlc.arg(status)
  AND (sqlc.narg(subject_id)::uuid IS NULL OR m.subject_id = sqlc.narg(subject_id)::uuid)
  AND (NOT sqlc.arg(no_subject)::boolean OR m.subject_id IS NULL)
  AND (sqlc.narg(kind)::text IS NULL OR m.kind = sqlc.narg(kind)::text)
  AND (sqlc.narg(uploader_id)::uuid IS NULL OR m.uploader_id = sqlc.narg(uploader_id)::uuid)
  AND (NOT sqlc.arg(inbox)::boolean OR m.needs_review)
  AND (cardinality(sqlc.arg(tag_ids)::uuid[]) = 0 OR (
        SELECT count(*) FROM material_tags mt
        WHERE mt.material_id = m.id AND mt.tag_id = ANY (sqlc.arg(tag_ids)::uuid[])
      ) = cardinality(sqlc.arg(tag_ids)::uuid[]))
  AND (sqlc.narg(query)::text IS NULL
       OR m.search @@ websearch_to_tsquery('russian', sqlc.narg(query)::text)
       OR m.title ILIKE '%' || sqlc.narg(like_pattern)::text || '%' ESCAPE '\'
       OR v.original_name ILIKE '%' || sqlc.narg(like_pattern)::text || '%' ESCAPE '\')
  AND (sqlc.narg(after_sort_at)::timestamptz IS NULL
       OR (m.sort_at, m.id) < (sqlc.narg(after_sort_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY m.sort_at DESC, m.id DESC
LIMIT sqlc.arg(max_rows);

-- name: ListMaterialTags :many
SELECT material_id, tag_id FROM material_tags WHERE material_id = ANY (sqlc.arg(material_ids)::uuid[]);

-- name: DeleteMaterialTags :exec
DELETE FROM material_tags WHERE material_id = $1;

-- name: InsertMaterialTags :exec
INSERT INTO material_tags (material_id, tag_id)
SELECT sqlc.arg(material_id)::uuid, unnest(sqlc.arg(tag_ids)::uuid[])
ON CONFLICT DO NOTHING;

-- name: CountInbox :one
SELECT count(*) FROM materials WHERE group_id = $1 AND needs_review AND status = 'ACTIVE';

-- name: UpdateMaterial :one
UPDATE materials
SET title = $2, description = $3, subject_id = $4, kind = $5, classification = $6,
    needs_review = $7, review_reason = $8
WHERE id = $1
RETURNING id, group_id, subject_id, uploader_id, title, description, kind, source, status, current_version_id,
          classification, needs_review, review_reason, download_count, sort_at, archived_by, archived_at,
          deleted_by, deleted_at, created_at, updated_at;

-- name: SetMaterialStatus :one
UPDATE materials
SET status = sqlc.arg(status),
    archived_by = CASE WHEN sqlc.arg(status) = 'ARCHIVED' THEN sqlc.narg(actor)::uuid
                       WHEN sqlc.arg(status) = 'ACTIVE' THEN NULL ELSE archived_by END,
    archived_at = CASE WHEN sqlc.arg(status) = 'ARCHIVED' THEN now()
                       WHEN sqlc.arg(status) = 'ACTIVE' THEN NULL ELSE archived_at END,
    deleted_by  = CASE WHEN sqlc.arg(status) = 'DELETED' THEN sqlc.narg(actor)::uuid ELSE NULL END,
    deleted_at  = CASE WHEN sqlc.arg(status) = 'DELETED' THEN now() ELSE NULL END
WHERE id = sqlc.arg(id)
RETURNING id, group_id, subject_id, uploader_id, title, description, kind, source, status, current_version_id,
          classification, needs_review, review_reason, download_count, sort_at, archived_by, archived_at,
          deleted_by, deleted_at, created_at, updated_at;

-- name: IncrementMaterialDownloads :exec
UPDATE materials SET download_count = download_count + 1 WHERE id = $1;

-- name: ListPurgeableMaterials :many
SELECT id, group_id, subject_id, uploader_id, title, description, kind, source, status, current_version_id,
       classification, needs_review, review_reason, download_count, sort_at, archived_by, archived_at,
       deleted_by, deleted_at, created_at, updated_at
FROM materials
WHERE status = 'DELETED' AND deleted_at < $1
ORDER BY deleted_at
LIMIT $2;

-- name: HardDeleteMaterial :exec
DELETE FROM materials WHERE id = $1;

-- name: CreateMaterialVersion :one
INSERT INTO material_versions (id, material_id, version_no, storage, storage_key, drive_file_id, drive_web_view_link,
                               drive_md5, drive_modified_time, drive_upload_status, original_name, mime, size_bytes,
                               sha256, scan_status, uploaded_by)
VALUES ($1, $2,
        COALESCE((SELECT max(version_no) FROM material_versions WHERE material_id = $2), 0) + 1,
        $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING *;

-- name: SetCurrentVersion :exec
UPDATE materials SET current_version_id = $2 WHERE id = $1;

-- name: GetMaterialVersion :one
SELECT * FROM material_versions WHERE id = $1;

-- name: ListMaterialVersions :many
SELECT * FROM material_versions WHERE material_id = $1 ORDER BY version_no DESC;

-- name: UpdateVersionFile :exec
UPDATE material_versions
SET original_name = $2, mime = $3, size_bytes = $4, drive_web_view_link = $5, drive_md5 = $6, drive_modified_time = $7
WHERE id = $1;

-- name: SetVersionDriveUpload :exec
UPDATE material_versions
SET drive_upload_status = $2, drive_upload_error = $3,
    drive_file_id = COALESCE(sqlc.narg(file_id), drive_file_id),
    drive_web_view_link = COALESCE(sqlc.narg(web_view_link), drive_web_view_link)
WHERE id = $1;

-- name: SetVersionHash :exec
UPDATE material_versions SET sha256 = $2 WHERE id = $1;
