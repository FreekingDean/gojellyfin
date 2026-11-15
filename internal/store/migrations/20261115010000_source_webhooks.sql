SET LOCAL lock_timeout = '30s';
SET LOCAL client_connection_check_interval = '10s';
ALTER TABLE "sources"
  ADD COLUMN IF NOT EXISTS "webhook_secret" character varying NULL,
  ADD COLUMN IF NOT EXISTS "webhook_registered_at" timestamptz NULL,
  ADD COLUMN IF NOT EXISTS "webhook_error" character varying NULL;
UPDATE "sources" SET "webhook_secret" = replace(gen_random_uuid()::text, '-', '') || replace(gen_random_uuid()::text, '-', '') WHERE "webhook_secret" IS NULL;
ALTER TABLE "sources" ALTER COLUMN "webhook_secret" SET NOT NULL;
