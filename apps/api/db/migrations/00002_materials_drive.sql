-- +goose Up

-- ---------------------------------------------------------------------------
-- Materials: files of a group, uploaded from the app or indexed from Drive
-- ---------------------------------------------------------------------------

CREATE TABLE materials (
  id                 uuid PRIMARY KEY,
  group_id           uuid NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
  subject_id         uuid REFERENCES subjects (id) ON DELETE SET NULL,
  uploader_id        uuid REFERENCES users (id) ON DELETE SET NULL,
  title              text NOT NULL,
  description        text NOT NULL DEFAULT '',
  kind               text NOT NULL DEFAULT 'OTHER'
                     CHECK (kind IN ('LECTURE', 'NOTES', 'REPORT', 'CALC', 'ASSIGNMENT', 'OTHER')),
  source             text NOT NULL CHECK (source IN ('UPLOAD', 'GDRIVE')),
  status             text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'ARCHIVED', 'DELETED')),
  current_version_id uuid,
  -- {subject_id, kind, confidence, method, signals, topic, semester}
  classification     jsonb NOT NULL DEFAULT '{}'::jsonb,
  needs_review       boolean NOT NULL DEFAULT false,
  review_reason      text CHECK (review_reason IN ('LOW_CONFIDENCE', 'REMOVED_FROM_DRIVE')),
  download_count     integer NOT NULL DEFAULT 0,
  -- ordering key of the feed: upload time or the file's creation time on Drive
  sort_at            timestamptz NOT NULL DEFAULT now(),
  search             tsvector GENERATED ALWAYS AS (
                       to_tsvector('russian', title || ' ' || description) || to_tsvector('simple', title)
                     ) STORED,
  archived_by        uuid REFERENCES users (id) ON DELETE SET NULL,
  archived_at        timestamptz,
  deleted_by         uuid REFERENCES users (id) ON DELETE SET NULL,
  deleted_at         timestamptz,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX materials_feed_idx ON materials (group_id, status, sort_at DESC, id DESC);
CREATE INDEX materials_subject_idx ON materials (group_id, subject_id, status);
CREATE INDEX materials_inbox_idx ON materials (group_id) WHERE needs_review AND status = 'ACTIVE';
CREATE INDEX materials_uploader_idx ON materials (uploader_id);
CREATE INDEX materials_search_gin ON materials USING gin (search);
CREATE INDEX materials_deleted_idx ON materials (deleted_at) WHERE status = 'DELETED';
CREATE TRIGGER materials_set_updated_at BEFORE UPDATE ON materials
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE material_versions (
  id                  uuid PRIMARY KEY,
  material_id         uuid NOT NULL REFERENCES materials (id) ON DELETE CASCADE,
  version_no          integer NOT NULL,
  storage             text NOT NULL CHECK (storage IN ('DRIVE', 'S3', 'LOCAL')),
  storage_key         text,
  cache_expires_at    timestamptz,
  drive_file_id       text,
  drive_web_view_link text,
  drive_md5           text,
  drive_modified_time timestamptz,
  -- copy to Drive requested from the app (FEATURE_DRIVE_UPLOAD)
  drive_upload_status text CHECK (drive_upload_status IN ('PENDING', 'DONE', 'FAILED')),
  drive_upload_error  text,
  original_name       text NOT NULL,
  mime                text NOT NULL,
  size_bytes          bigint NOT NULL DEFAULT 0,
  sha256              text,
  scan_status         text NOT NULL DEFAULT 'SKIPPED'
                      CHECK (scan_status IN ('PENDING', 'CLEAN', 'INFECTED', 'SKIPPED')),
  text_key            text,
  preview_key         text,
  uploaded_by         uuid REFERENCES users (id) ON DELETE SET NULL,
  created_at          timestamptz NOT NULL DEFAULT now(),
  UNIQUE (material_id, version_no)
);
CREATE INDEX material_versions_drive_file_idx ON material_versions (drive_file_id) WHERE drive_file_id IS NOT NULL;
CREATE INDEX material_versions_cache_idx ON material_versions (cache_expires_at) WHERE cache_expires_at IS NOT NULL;
CREATE INDEX material_versions_sha_idx ON material_versions (sha256) WHERE sha256 IS NOT NULL;

ALTER TABLE materials
  ADD CONSTRAINT materials_current_version_fk
  FOREIGN KEY (current_version_id) REFERENCES material_versions (id) ON DELETE SET NULL;

CREATE TABLE material_tags (
  material_id uuid NOT NULL REFERENCES materials (id) ON DELETE CASCADE,
  tag_id      uuid NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
  PRIMARY KEY (material_id, tag_id)
);
CREATE INDEX material_tags_tag_idx ON material_tags (tag_id);

-- Pending direct uploads (presigned PUT to S3 or signed PUT to the API for
-- the local driver). Completed uploads become materials.
CREATE TABLE uploads (
  id           uuid PRIMARY KEY,
  group_id     uuid NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
  user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  storage      text NOT NULL CHECK (storage IN ('S3', 'LOCAL')),
  storage_key  text NOT NULL,
  file_name    text NOT NULL,
  mime         text NOT NULL,
  size_bytes   bigint NOT NULL,
  -- material form captured at start: {title, description, subject_id, kind, tag_ids, to_drive}
  meta         jsonb NOT NULL DEFAULT '{}'::jsonb,
  status       text NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'COMPLETED', 'EXPIRED')),
  material_id  uuid REFERENCES materials (id) ON DELETE SET NULL,
  expires_at   timestamptz NOT NULL,
  completed_at timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX uploads_expiry_idx ON uploads (expires_at) WHERE status = 'PENDING';

-- ---------------------------------------------------------------------------
-- Google Drive: one service-account connection per group and its file index
-- ---------------------------------------------------------------------------

CREATE TABLE drive_connections (
  id                 uuid PRIMARY KEY,
  group_id           uuid NOT NULL UNIQUE REFERENCES groups (id) ON DELETE CASCADE,
  mode               text NOT NULL DEFAULT 'SERVICE_ACCOUNT' CHECK (mode IN ('SERVICE_ACCOUNT')),
  root_folder_id     text NOT NULL,
  root_folder_name   text NOT NULL DEFAULT '',
  drive_id           text,
  status             text NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'SYNCING', 'OK', 'ERROR')),
  last_error         text,
  sync_started_at    timestamptz,
  last_sync_at       timestamptz,
  last_full_scan_at  timestamptz,
  changes_page_token text,
  sync_interval_sec  integer NOT NULL,
  writable           boolean NOT NULL DEFAULT false,
  created_by         uuid REFERENCES users (id) ON DELETE SET NULL,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER drive_connections_set_updated_at BEFORE UPDATE ON drive_connections
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE drive_items (
  id             uuid PRIMARY KEY,
  connection_id  uuid NOT NULL REFERENCES drive_connections (id) ON DELETE CASCADE,
  drive_file_id  text NOT NULL,
  parent_id      text,
  -- folder names from the root (exclusive) to the parent, joined with "/"
  path_cache     text NOT NULL DEFAULT '',
  name           text NOT NULL,
  mime           text NOT NULL,
  is_folder      boolean NOT NULL DEFAULT false,
  md5            text,
  size_bytes     bigint,
  modified_time  timestamptz,
  web_view_link  text,
  material_id    uuid REFERENCES materials (id) ON DELETE SET NULL,
  state          text NOT NULL DEFAULT 'NEW'
                 CHECK (state IN ('NEW', 'LINKED', 'IMPORTED', 'SKIPPED', 'ERROR', 'DELETED')),
  classification jsonb NOT NULL DEFAULT '{}'::jsonb,
  last_error     text,
  seen_at        timestamptz NOT NULL DEFAULT now(),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (connection_id, drive_file_id)
);
CREATE INDEX drive_items_parent_idx ON drive_items (connection_id, parent_id);
CREATE INDEX drive_items_material_idx ON drive_items (material_id) WHERE material_id IS NOT NULL;
CREATE TRIGGER drive_items_set_updated_at BEFORE UPDATE ON drive_items
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS drive_items;
DROP TABLE IF EXISTS drive_connections;
DROP TABLE IF EXISTS uploads;
DROP TABLE IF EXISTS material_tags;
ALTER TABLE IF EXISTS materials DROP CONSTRAINT IF EXISTS materials_current_version_fk;
DROP TABLE IF EXISTS material_versions;
DROP TABLE IF EXISTS materials;
