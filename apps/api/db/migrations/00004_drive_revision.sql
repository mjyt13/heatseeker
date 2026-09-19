-- +goose Up

-- Drive revision a version was indexed from (files.headRevisionId). Older
-- versions of a Drive file are served from this revision, while Google keeps
-- it; versions indexed before this column are matched by md5 on first open.
ALTER TABLE material_versions ADD COLUMN drive_revision_id text;

-- +goose Down
ALTER TABLE material_versions DROP COLUMN drive_revision_id;
