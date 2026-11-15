SET LOCAL lock_timeout = '30s';
SET LOCAL client_connection_check_interval = '10s';
ALTER TABLE "item_sources" ADD COLUMN IF NOT EXISTS "keyframes" jsonb NULL;
UPDATE "item_sources" SET "probed_at" = NULL WHERE "keyframes" IS NULL AND "probed_at" IS NOT NULL AND EXISTS (SELECT 1 FROM "media_streams" WHERE "media_streams"."item_source_id" = "item_sources"."id" AND "media_streams"."kind" = 'Video');
