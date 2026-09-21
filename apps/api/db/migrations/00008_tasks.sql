-- +goose Up

-- Tasks: homework and group to-dos. Anyone may create one; moderators and the
-- headman pin tasks and move the group status. Personal progress of each
-- member lives in task_assignments.
CREATE TABLE tasks (
  id               uuid PRIMARY KEY,
  group_id         uuid NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
  subject_id       uuid REFERENCES subjects (id) ON DELETE SET NULL,
  created_by       uuid REFERENCES users (id) ON DELETE SET NULL,
  -- client_id makes creation idempotent when the phone repeats the request.
  client_id        uuid,
  title            text NOT NULL,
  description      text NOT NULL DEFAULT '',
  kind             text NOT NULL DEFAULT 'GROUP' CHECK (kind IN ('TEACHER', 'GROUP', 'PERSONAL')),
  status           text NOT NULL DEFAULT 'TODO' CHECK (status IN ('TODO', 'IN_PROGRESS', 'IN_REVIEW', 'DONE', 'CANCELLED')),
  priority         text NOT NULL DEFAULT 'NORMAL' CHECK (priority IN ('LOW', 'NORMAL', 'HIGH')),
  assign_mode      text NOT NULL DEFAULT 'ALL' CHECK (assign_mode IN ('ALL', 'SELECTED', 'SELF')),
  visibility       text NOT NULL DEFAULT 'GROUP' CHECK (visibility IN ('GROUP', 'PRIVATE')),
  due_at           timestamptz,
  -- Deadline offsets (minutes before due_at) the scanner has already announced.
  notified_offsets integer[] NOT NULL DEFAULT '{}',
  pinned_by        uuid REFERENCES users (id) ON DELETE SET NULL,
  pinned_at        timestamptz,
  completed_at     timestamptz,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz
);
CREATE INDEX tasks_group_due_idx ON tasks (group_id, due_at) WHERE deleted_at IS NULL;
CREATE INDEX tasks_group_status_idx ON tasks (group_id, status) WHERE deleted_at IS NULL;
CREATE INDEX tasks_subject_idx ON tasks (subject_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX tasks_client_uidx ON tasks (group_id, client_id) WHERE client_id IS NOT NULL;
CREATE TRIGGER tasks_set_updated_at BEFORE UPDATE ON tasks
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Personal progress. A row appears when a member is assigned (SELECTED) or
-- first moves their own status.
CREATE TABLE task_assignments (
  task_id      uuid NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
  user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  status       text NOT NULL DEFAULT 'TODO' CHECK (status IN ('TODO', 'IN_PROGRESS', 'IN_REVIEW', 'DONE', 'CANCELLED')),
  completed_at timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (task_id, user_id)
);
CREATE INDEX task_assignments_user_idx ON task_assignments (user_id, status);
CREATE TRIGGER task_assignments_set_updated_at BEFORE UPDATE ON task_assignments
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Materials attached to a task (the assignment file, a template, an example).
CREATE TABLE task_attachments (
  task_id     uuid NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
  material_id uuid NOT NULL REFERENCES materials (id) ON DELETE CASCADE,
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (task_id, material_id)
);
CREATE INDEX task_attachments_material_idx ON task_attachments (material_id);

-- +goose Down
DROP TABLE IF EXISTS task_attachments;
DROP TABLE IF EXISTS task_assignments;
DROP TABLE IF EXISTS tasks;
