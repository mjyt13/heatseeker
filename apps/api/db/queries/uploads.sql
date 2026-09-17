-- name: CreateUpload :one
INSERT INTO uploads (id, group_id, user_id, storage, storage_key, file_name, mime, size_bytes, meta, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetUpload :one
SELECT * FROM uploads WHERE id = $1;

-- name: CompleteUpload :execrows
UPDATE uploads SET status = 'COMPLETED', material_id = $2, completed_at = now()
WHERE id = $1 AND status = 'PENDING';

-- name: ListExpiredUploads :many
SELECT * FROM uploads WHERE status = 'PENDING' AND expires_at < $1 ORDER BY expires_at LIMIT $2;

-- name: MarkUploadExpired :exec
UPDATE uploads SET status = 'EXPIRED' WHERE id = $1 AND status = 'PENDING';
