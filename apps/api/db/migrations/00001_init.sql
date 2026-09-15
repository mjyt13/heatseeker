-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- Users and authentication
-- ---------------------------------------------------------------------------

CREATE TABLE users (
  id            uuid PRIMARY KEY,
  name          text NOT NULL,
  email         text,
  password_hash text,
  locale        text NOT NULL DEFAULT 'ru',
  timezone      text NOT NULL DEFAULT 'UTC',
  avatar_key    text,
  global_role   text NOT NULL DEFAULT 'USER' CHECK (global_role IN ('USER', 'SUPERADMIN')),
  settings      jsonb NOT NULL DEFAULT '{}'::jsonb,
  secured_at    timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz
);
CREATE UNIQUE INDEX users_email_lower_uidx ON users (lower(email)) WHERE email IS NOT NULL;
CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE auth_identities (
  id                uuid PRIMARY KEY,
  user_id           uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  provider          text NOT NULL CHECK (provider IN ('GOOGLE', 'APPLE')),
  provider_user_id  text NOT NULL,
  email             text,
  refresh_token_enc bytea,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_user_id)
);
CREATE INDEX auth_identities_user_idx ON auth_identities (user_id);
CREATE TRIGGER auth_identities_set_updated_at BEFORE UPDATE ON auth_identities
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE devices (
  id                uuid PRIMARY KEY,
  user_id           uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  platform          text NOT NULL CHECK (platform IN ('IOS', 'ANDROID', 'WEB')),
  name              text,
  push_provider     text CHECK (push_provider IN ('EXPO', 'WEBPUSH')),
  push_token        text,
  push_subscription jsonb,
  enabled           boolean NOT NULL DEFAULT true,
  last_seen_at      timestamptz NOT NULL DEFAULT now(),
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX devices_user_idx ON devices (user_id);
CREATE TRIGGER devices_set_updated_at BEFORE UPDATE ON devices
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE refresh_tokens (
  id          uuid PRIMARY KEY,
  user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  device_id   uuid REFERENCES devices (id) ON DELETE SET NULL,
  token_hash  text NOT NULL UNIQUE,
  expires_at  timestamptz NOT NULL,
  revoked_at  timestamptz,
  replaced_by uuid,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);
CREATE INDEX refresh_tokens_expires_idx ON refresh_tokens (expires_at);

-- ---------------------------------------------------------------------------
-- Groups, membership, invites
-- ---------------------------------------------------------------------------

CREATE TABLE groups (
  id          uuid PRIMARY KEY,
  name        text NOT NULL,
  slug        text NOT NULL UNIQUE,
  kind        text NOT NULL DEFAULT 'MASTERS' CHECK (kind IN ('MASTERS', 'DPO', 'OTHER')),
  join_policy text NOT NULL DEFAULT 'OPEN' CHECK (join_policy IN ('OPEN', 'INVITE', 'APPROVAL')),
  join_code   text UNIQUE,
  public_read boolean NOT NULL DEFAULT false,
  media_mode  text NOT NULL DEFAULT 'CACHE' CHECK (media_mode IN ('LINK', 'CACHE', 'IMPORT')),
  settings    jsonb NOT NULL DEFAULT '{}'::jsonb,
  last_seq    bigint NOT NULL DEFAULT 0,
  created_by  uuid NOT NULL REFERENCES users (id),
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz
);
CREATE TRIGGER groups_set_updated_at BEFORE UPDATE ON groups
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE memberships (
  id         uuid PRIMARY KEY,
  user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  group_id   uuid NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
  roles      text[] NOT NULL DEFAULT '{STUDENT}',
  status     text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'PENDING', 'BANNED')),
  joined_via text NOT NULL DEFAULT 'LINK' CHECK (joined_via IN ('LINK', 'INVITE', 'APPROVAL', 'CREATOR')),
  joined_at  timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, group_id)
);
CREATE INDEX memberships_group_idx ON memberships (group_id);
CREATE TRIGGER memberships_set_updated_at BEFORE UPDATE ON memberships
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE invites (
  id         uuid PRIMARY KEY,
  group_id   uuid NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
  code       text NOT NULL UNIQUE,
  roles      text[] NOT NULL DEFAULT '{STUDENT}',
  expires_at timestamptz,
  max_uses   integer,
  uses       integer NOT NULL DEFAULT 0,
  created_by uuid NOT NULL REFERENCES users (id),
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX invites_group_idx ON invites (group_id);

-- ---------------------------------------------------------------------------
-- Subjects and tags
-- ---------------------------------------------------------------------------

CREATE TABLE subjects (
  id              uuid PRIMARY KEY,
  group_id        uuid NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
  name            text NOT NULL,
  short_name      text,
  teacher         text,
  teacher_contact text,
  color           text,
  semester        text,
  aliases         text[] NOT NULL DEFAULT '{}',
  sort_order      integer NOT NULL DEFAULT 0,
  created_by      uuid REFERENCES users (id) ON DELETE SET NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  archived_at     timestamptz
);
CREATE INDEX subjects_group_idx ON subjects (group_id, sort_order) WHERE archived_at IS NULL;
CREATE INDEX subjects_aliases_gin ON subjects USING gin (aliases);
CREATE TRIGGER subjects_set_updated_at BEFORE UPDATE ON subjects
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE tags (
  id         uuid PRIMARY KEY,
  group_id   uuid NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
  name       text NOT NULL,
  slug       text NOT NULL,
  color      text,
  kind       text NOT NULL DEFAULT 'CUSTOM' CHECK (kind IN ('SUBJECT', 'TOPIC', 'TYPE', 'SYSTEM', 'CUSTOM')),
  subject_id uuid REFERENCES subjects (id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (group_id, slug)
);
CREATE UNIQUE INDEX tags_subject_uidx ON tags (subject_id) WHERE subject_id IS NOT NULL;
CREATE TRIGGER tags_set_updated_at BEFORE UPDATE ON tags
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Group event log: single source for realtime, offline sync and activity feed
-- ---------------------------------------------------------------------------

CREATE TABLE group_events (
  id          uuid PRIMARY KEY,
  group_id    uuid NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
  seq         bigint NOT NULL,
  kind        text NOT NULL,
  actor_id    uuid REFERENCES users (id) ON DELETE SET NULL,
  entity_type text NOT NULL,
  entity_id   uuid,
  payload     jsonb NOT NULL DEFAULT '{}'::jsonb,
  audit       boolean NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (group_id, seq)
);
CREATE INDEX group_events_created_idx ON group_events (created_at);

-- +goose Down
DROP TABLE IF EXISTS group_events;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS subjects;
DROP TABLE IF EXISTS invites;
DROP TABLE IF EXISTS memberships;
DROP TABLE IF EXISTS groups;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS auth_identities;
DROP TABLE IF EXISTS users;
DROP FUNCTION IF EXISTS set_updated_at();
