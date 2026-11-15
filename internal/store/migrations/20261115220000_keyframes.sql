SET LOCAL lock_timeout = '30s';
SET LOCAL client_connection_check_interval = '10s';
CREATE TABLE IF NOT EXISTS "keyframe_indexes" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "ticks" jsonb NOT NULL,
  "item_source_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "keyframe_indexes_item_sources_keyframes" FOREIGN KEY ("item_source_id") REFERENCES "item_sources" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS "keyframe_indexes_item_source_id_key" ON "keyframe_indexes" ("item_source_id");
UPDATE "item_sources" SET "probed_at" = NULL
WHERE "probed_at" IS NOT NULL
  AND "id" IN (SELECT "item_source_id" FROM "media_streams" WHERE "kind" = 'Video')
  AND "id" NOT IN (SELECT "item_source_id" FROM "keyframe_indexes");
