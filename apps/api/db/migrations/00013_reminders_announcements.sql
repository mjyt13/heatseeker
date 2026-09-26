-- +goose Up
-- Announcements (docs/PLAN.md §8.2): the headman speaks to the whole group.
-- They are group news, so they go through the event log like everything else
-- and land in the activity feed.
CREATE TABLE announcements (
  id         uuid PRIMARY KEY,
  group_id   uuid NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
  author_id  uuid REFERENCES users (id) ON DELETE SET NULL,
  title      text NOT NULL,
  body       text NOT NULL DEFAULT '',
  -- Urgent announcements reach the phone even during quiet hours.
  urgent     boolean NOT NULL DEFAULT false,
  -- Pinned ones stay on top of the list until taken down.
  pinned     boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE INDEX announcements_group_idx ON announcements (group_id, pinned DESC, created_at DESC)
  WHERE deleted_at IS NULL;
CREATE TRIGGER announcements_set_updated_at BEFORE UPDATE ON announcements
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Custom reminders: one person's own note to self ("hand the materials to the
-- teacher on Thursday"). The text is personal, so it never enters the group
-- log — the notifier writes the notification straight to its owner.
CREATE TABLE reminders (
  id           uuid PRIMARY KEY,
  group_id     uuid NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
  user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  title        text NOT NULL,
  note         text NOT NULL DEFAULT '',
  remind_at    timestamptz NOT NULL,
  -- A repeat moves remind_at forward instead of closing the reminder.
  repeat       text NOT NULL DEFAULT 'NONE' CHECK (repeat IN ('NONE', 'DAILY', 'WEEKLY', 'MONTHLY')),
  -- Optional link to what it is about, for the deep link.
  target_type  text CHECK (target_type IN ('TASK', 'MATERIAL', 'SCHEDULE')),
  target_id    uuid,
  -- SCHEDULED — waiting; SENT — fired and still open; DONE — closed by its owner.
  status       text NOT NULL DEFAULT 'SCHEDULED' CHECK (status IN ('SCHEDULED', 'SENT', 'DONE')),
  last_fired_at timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT reminders_target_pair CHECK ((target_type IS NULL) = (target_id IS NULL))
);
CREATE INDEX reminders_due_idx ON reminders (remind_at) WHERE status = 'SCHEDULED';
CREATE INDEX reminders_user_idx ON reminders (user_id, group_id, remind_at DESC);
CREATE TRIGGER reminders_set_updated_at BEFORE UPDATE ON reminders
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS reminders;
DROP TABLE IF EXISTS announcements;
