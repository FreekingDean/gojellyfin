SET LOCAL lock_timeout = '30s';
SET LOCAL client_connection_check_interval = '10s';
-- Modify "item_sources" table
ALTER TABLE "item_sources" ADD COLUMN IF NOT EXISTS "lyrics" text NULL;
