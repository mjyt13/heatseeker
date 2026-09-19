-- A separate database for integration tests, so they never touch dev data.
-- Runs only when the pgdata volume is created; on an existing volume:
--   podman exec heatseeker-dev-postgres-1 createdb -U heatseeker heatseeker_test
CREATE DATABASE heatseeker_test OWNER heatseeker;
