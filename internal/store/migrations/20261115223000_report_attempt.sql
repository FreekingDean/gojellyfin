SET LOCAL lock_timeout = '30s';
SET LOCAL client_connection_check_interval = '10s';
ALTER TABLE "reports" ADD COLUMN IF NOT EXISTS "attempt" jsonb NULL;
