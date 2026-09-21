-- +goose Up
-- Discussions (docs/PLAN.md §7.1, D11). A thread belongs to a target — a
-- subject, the whole group, a material or a task — and is created with its
-- first message (D40), so reading an empty discussion writes nothing.
CREATE TABLE threads (
  id              uuid PRIMARY KEY,
  group_id        uuid NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
  target_type     text NOT NULL CHECK (target_type IN ('SUBJECT', 'LESSON', 'MATERIAL', 'TASK', 'PROPOSAL', 'GENERAL')),
  -- The subject, material or task id; the group id for GENERAL.
  target_id       uuid NOT NULL,
  -- Denormalised for navigation by subject (a material's or task's subject).
  subject_id      uuid REFERENCES subjects (id) ON DELETE SET NULL,
  title           text,
  created_by      uuid REFERENCES users (id) ON DELETE SET NULL,
  message_count   integer NOT NULL DEFAULT 0,
  last_message_at timestamptz,
  -- seq of the newest message, compared with thread_reads for unread counts.
  last_seq        bigint NOT NULL DEFAULT 0,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX threads_target_uidx ON threads (group_id, target_type, target_id);
CREATE INDEX threads_group_subject_idx ON threads (group_id, subject_id, last_message_at DESC);
CREATE TRIGGER threads_set_updated_at BEFORE UPDATE ON threads
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE messages (
  id                uuid PRIMARY KEY,
  group_id          uuid NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
  thread_id         uuid NOT NULL REFERENCES threads (id) ON DELETE CASCADE,
  author_id         uuid REFERENCES users (id) ON DELETE SET NULL,
  -- Idempotency of offline sending: the phone repeats the request with the same id.
  client_id         uuid NOT NULL,
  -- seq of the message.created event in group_events.
  seq               bigint NOT NULL,
  body              text NOT NULL,
  reply_to_id       uuid REFERENCES messages (id) ON DELETE SET NULL,
  edited_at         timestamptz,
  deleted_at        timestamptz,
  hidden_for_all_by uuid REFERENCES users (id) ON DELETE SET NULL,
  hidden_for_all_at timestamptz,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX messages_client_uidx ON messages (thread_id, client_id);
CREATE INDEX messages_thread_seq_idx ON messages (thread_id, seq);
CREATE INDEX messages_group_seq_idx ON messages (group_id, seq);
CREATE TRIGGER messages_set_updated_at BEFORE UPDATE ON messages
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Hidden "for me": the message stays for everyone else.
CREATE TABLE message_hides (
  message_id uuid NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (message_id, user_id)
);
CREATE INDEX message_hides_user_idx ON message_hides (user_id, created_at DESC);

CREATE TABLE thread_reads (
  thread_id     uuid NOT NULL REFERENCES threads (id) ON DELETE CASCADE,
  user_id       uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  last_read_seq bigint NOT NULL DEFAULT 0,
  updated_at    timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (thread_id, user_id)
);

-- +goose Down
DROP TABLE IF EXISTS thread_reads;
DROP TABLE IF EXISTS message_hides;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS threads;
