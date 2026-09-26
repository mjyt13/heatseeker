-- +goose Up
-- Whether urgent announcements may break through this member's quiet hours.
-- Off by default: the night belongs to the member, the notification waits in
-- the list till morning (D51).
ALTER TABLE notification_settings
  ADD COLUMN urgent_in_quiet boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE notification_settings DROP COLUMN IF EXISTS urgent_in_quiet;
