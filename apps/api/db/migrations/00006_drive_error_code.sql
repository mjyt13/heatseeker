-- +goose Up

-- Machine-readable code of the last sync failure (domain error codes, e.g.
-- folder_not_shared), so clients show a translated message instead of the
-- raw text from Google kept in last_error.
ALTER TABLE drive_connections ADD COLUMN last_error_code text;

-- +goose Down
ALTER TABLE drive_connections DROP COLUMN last_error_code;
