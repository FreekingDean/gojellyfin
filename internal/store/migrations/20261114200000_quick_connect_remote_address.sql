-- Modify "quick_connect_requests" table
ALTER TABLE "quick_connect_requests" ADD COLUMN IF NOT EXISTS "remote_address" character varying NOT NULL DEFAULT '';
