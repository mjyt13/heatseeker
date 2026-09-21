-- +goose Up
-- A file uploaded into a task can stay with that task (D43): it is not in the
-- group's feed, search or Inbox and is seen by those who see the task. NULL —
-- an ordinary group material; sharing a file clears the column.
ALTER TABLE materials ADD COLUMN task_id uuid REFERENCES tasks (id) ON DELETE SET NULL;
CREATE INDEX materials_task_idx ON materials (task_id) WHERE task_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS materials_task_idx;
ALTER TABLE materials DROP COLUMN IF EXISTS task_id;
