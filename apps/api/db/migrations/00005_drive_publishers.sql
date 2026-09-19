-- +goose Up

-- The Google account that publishes uploads to the group's Drive folder (D34):
-- a service account has no storage quota in "My Drive", so files are created
-- on behalf of the head's account connected through OAuth. The refresh token
-- is sealed with APP_ENCRYPTION_KEY (AES-GCM, bound to the group id).
CREATE TABLE drive_publishers (
  group_id          uuid PRIMARY KEY REFERENCES groups (id) ON DELETE CASCADE,
  google_email      text NOT NULL,
  refresh_token_enc bytea NOT NULL,
  scopes            text NOT NULL DEFAULT '',
  -- set when Google rejects the token (revoked access, changed password)
  last_error        text,
  connected_by      uuid REFERENCES users (id) ON DELETE SET NULL,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER drive_publishers_set_updated_at BEFORE UPDATE ON drive_publishers
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE drive_publishers;
