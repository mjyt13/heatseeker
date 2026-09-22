-- +goose Up

-- Schedule: one-off classes and weekly series (every week or every other
-- week — "числитель / знаменатель"). Occurrences are not stored: they are
-- expanded from the series on read, in the event's own time zone, so a class
-- stays at 10:00 local time whatever the offset.
CREATE TABLE schedule_events (
  id          uuid PRIMARY KEY,
  group_id    uuid NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
  subject_id  uuid REFERENCES subjects (id) ON DELETE SET NULL,
  -- client_id makes creation idempotent when the phone repeats the request.
  client_id   uuid,
  title       text NOT NULL DEFAULT '',
  kind        text NOT NULL DEFAULT 'LECTURE'
              CHECK (kind IN ('LECTURE', 'SEMINAR', 'LAB', 'EXAM', 'CONSULTATION', 'OTHER')),
  -- The first occurrence; later ones repeat its local wall time.
  starts_at   timestamptz NOT NULL,
  ends_at     timestamptz NOT NULL,
  timezone    text NOT NULL,
  location    text NOT NULL DEFAULT '',
  teacher     text NOT NULL DEFAULT '',
  note        text NOT NULL DEFAULT '',
  -- RFC 5545 subset: FREQ=WEEKLY;INTERVAL=n;BYDAY=MO,TH. NULL — one-off.
  rrule       text,
  -- Last local date of the series, inclusive. NULL — no end.
  rrule_until date,
  created_by  uuid REFERENCES users (id) ON DELETE SET NULL,
  updated_by  uuid REFERENCES users (id) ON DELETE SET NULL,
  -- Optimistic locking: an edit names the version it started from.
  version     integer NOT NULL DEFAULT 1,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,
  CHECK (ends_at > starts_at)
);
CREATE INDEX schedule_events_group_idx ON schedule_events (group_id, starts_at) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX schedule_events_client_uidx ON schedule_events (group_id, client_id) WHERE client_id IS NOT NULL;
CREATE TRIGGER schedule_events_set_updated_at BEFORE UPDATE ON schedule_events
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One occurrence of a series (or a one-off class) cancelled or changed:
-- another time, room or teacher. Keyed by the local date it was planned for.
CREATE TABLE schedule_exceptions (
  event_id      uuid NOT NULL REFERENCES schedule_events (id) ON DELETE CASCADE,
  original_date date NOT NULL,
  kind          text NOT NULL CHECK (kind IN ('CANCELLED', 'CHANGED')),
  -- Overrides; NULL keeps the series value.
  starts_at     timestamptz,
  ends_at       timestamptz,
  location      text,
  teacher       text,
  -- Why: "перенос из-за праздника".
  note          text NOT NULL DEFAULT '',
  updated_by    uuid REFERENCES users (id) ON DELETE SET NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (event_id, original_date),
  CHECK ((starts_at IS NULL) = (ends_at IS NULL)),
  CHECK (ends_at IS NULL OR ends_at > starts_at)
);
-- Occurrences moved into a window from a date outside it.
CREATE INDEX schedule_exceptions_moved_idx ON schedule_exceptions (starts_at) WHERE starts_at IS NOT NULL;
CREATE TRIGGER schedule_exceptions_set_updated_at BEFORE UPDATE ON schedule_exceptions
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE schedule_exceptions;
DROP TABLE schedule_events;
