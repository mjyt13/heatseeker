-- +goose Up
-- Notifications (docs/PLAN.md §8). The group event log is the only source:
-- a reader walks it per group from notifier_cursors and writes one row per
-- recipient here. Delivery to a device is a separate row, so a failed push
-- never loses the in-app notification.
CREATE TABLE notifications (
  id          uuid PRIMARY KEY,
  user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  group_id    uuid NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
  type        text NOT NULL CHECK (type IN (
                'MESSAGE_NEW', 'MESSAGE_REPLY', 'MATERIAL_ADDED', 'MATERIAL_BATCH',
                'TASK_CREATED', 'TASK_PINNED', 'TASK_DUE_SOON', 'TASK_OVERDUE', 'TASK_STATUS_CHANGED',
                'SCHEDULE_CHANGED', 'MEMBER_JOINED', 'ANNOUNCEMENT', 'REMINDER', 'PROPOSAL_NEW', 'MODERATION')),
  title       text NOT NULL,
  body        text NOT NULL DEFAULT '',
  -- Where tapping it leads: {"screen":"thread","thread_id":...} and friends.
  data        jsonb NOT NULL DEFAULT '{}'::jsonb,
  -- seq of the group event that produced it; the cursor never goes back.
  seq         bigint NOT NULL DEFAULT 0,
  -- One notification per user per meaningful fact; a repeated run is a no-op.
  dedupe_key  text NOT NULL,
  read_at     timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX notifications_dedupe_uidx ON notifications (user_id, dedupe_key);
CREATE INDEX notifications_user_idx ON notifications (user_id, group_id, created_at DESC, id);
CREATE INDEX notifications_unread_idx ON notifications (user_id, group_id) WHERE read_at IS NULL;

-- How far the reader has walked each group's log.
CREATE TABLE notifier_cursors (
  group_id   uuid PRIMARY KEY REFERENCES groups (id) ON DELETE CASCADE,
  last_seq   bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- What a member wants to hear about in a group. A missing row means the
-- default for the type and the group kind (DPO — everything off but
-- announcements, docs/PLAN.md §8.2).
CREATE TABLE notification_prefs (
  user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  group_id   uuid NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
  type       text NOT NULL,
  enabled    boolean NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, group_id, type)
);
CREATE TRIGGER notification_prefs_set_updated_at BEFORE UPDATE ON notification_prefs
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Silence with an end date (never forever): a subject, a discussion, one type
-- or the whole group.
CREATE TABLE notification_mutes (
  id         uuid PRIMARY KEY,
  user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  group_id   uuid NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
  scope_type text NOT NULL CHECK (scope_type IN ('GROUP', 'SUBJECT', 'THREAD', 'TYPE')),
  -- The subject, thread or group id as text; the notification type for TYPE.
  scope_id   text NOT NULL,
  until      timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX notification_mutes_uidx ON notification_mutes (user_id, group_id, scope_type, scope_id);
CREATE INDEX notification_mutes_user_idx ON notification_mutes (user_id, until);

-- Personal delivery settings: quiet hours are read in the user's timezone.
CREATE TABLE notification_settings (
  user_id      uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
  push_enabled boolean NOT NULL DEFAULT true,
  -- Minutes from midnight in the user's timezone; NULL — no quiet hours.
  -- from > to means the window crosses midnight (23:00–08:00).
  quiet_from   smallint CHECK (quiet_from BETWEEN 0 AND 1439),
  quiet_to     smallint CHECK (quiet_to BETWEEN 0 AND 1439),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER notification_settings_set_updated_at BEFORE UPDATE ON notification_settings
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One attempt to put a notification on a device.
CREATE TABLE notification_deliveries (
  id              uuid PRIMARY KEY,
  notification_id uuid NOT NULL REFERENCES notifications (id) ON DELETE CASCADE,
  channel         text NOT NULL CHECK (channel IN ('PUSH', 'WEBPUSH', 'EMAIL', 'TELEGRAM')),
  device_id       uuid REFERENCES devices (id) ON DELETE SET NULL,
  status          text NOT NULL CHECK (status IN ('QUEUED', 'SENT', 'FAILED', 'SKIPPED')),
  error           text,
  sent_at         timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notification_deliveries_notification_idx ON notification_deliveries (notification_id);

-- +goose Down
DROP TABLE IF EXISTS notification_deliveries;
DROP TABLE IF EXISTS notification_settings;
DROP TABLE IF EXISTS notification_mutes;
DROP TABLE IF EXISTS notification_prefs;
DROP TABLE IF EXISTS notifier_cursors;
DROP TABLE IF EXISTS notifications;
