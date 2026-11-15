SET LOCAL lock_timeout = '30s';
SET LOCAL client_connection_check_interval = '10s';
-- Modify "persons" table
ALTER TABLE "persons" ADD COLUMN IF NOT EXISTS "image_url" character varying NULL;
