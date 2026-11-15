SET LOCAL lock_timeout = '30s';
SET LOCAL client_connection_check_interval = '10s';
ALTER TABLE "item_sources" ADD COLUMN IF NOT EXISTS "segments_detected_at" timestamptz NULL;
UPDATE "item_sources" SET "probed_at" = NULL WHERE "segments_detected_at" IS NULL AND "probed_at" IS NOT NULL;
CREATE TABLE IF NOT EXISTS "media_segments" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "kind" character varying NOT NULL,
  "origin" character varying NOT NULL,
  "start_ticks" bigint NOT NULL,
  "end_ticks" bigint NOT NULL,
  "item_source_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "media_segments_item_sources_segments" FOREIGN KEY ("item_source_id") REFERENCES "item_sources" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS "mediasegment_item_source_id_origin_kind" ON "media_segments" ("item_source_id", "origin", "kind");
