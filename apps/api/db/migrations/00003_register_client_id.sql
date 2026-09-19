-- +goose Up

-- Idempotent sign-up: the app sends a random client_id generated when the
-- welcome screen opens; a repeated request (lost response, double tap) returns
-- the same account instead of creating another one.
ALTER TABLE users ADD COLUMN register_client_id uuid;
CREATE UNIQUE INDEX users_register_client_id_uidx ON users (register_client_id)
  WHERE register_client_id IS NOT NULL;

-- +goose Down
DROP INDEX users_register_client_id_uidx;
ALTER TABLE users DROP COLUMN register_client_id;
