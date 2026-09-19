-- +goose Up

-- PDF previews of office files (pptx/docx/xlsx…), converted by the worker
-- through Gotenberg/LibreOffice and stored next to the files (preview_key).
-- NULL status: not requested yet.
ALTER TABLE material_versions
  ADD COLUMN preview_status text CHECK (preview_status IN ('PENDING', 'READY', 'FAILED', 'SKIPPED')),
  ADD COLUMN preview_error  text;

-- +goose Down
ALTER TABLE material_versions DROP COLUMN preview_error, DROP COLUMN preview_status;
