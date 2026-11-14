-- Modify "users" table
ALTER TABLE "users" ADD COLUMN IF NOT EXISTS "can_request" boolean NOT NULL DEFAULT true, ADD COLUMN IF NOT EXISTS "requests_need_approval" boolean NOT NULL DEFAULT false, ADD COLUMN IF NOT EXISTS "request_quota" bigint NULL;
-- Modify "requests" table
ALTER TABLE "requests" ADD COLUMN IF NOT EXISTS "status" character varying NOT NULL DEFAULT 'Approved', ADD COLUMN IF NOT EXISTS "tvdb_id" bigint NOT NULL DEFAULT 0;
