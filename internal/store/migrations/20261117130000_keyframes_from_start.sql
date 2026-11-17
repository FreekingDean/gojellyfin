SET LOCAL lock_timeout = '30s';
SET LOCAL client_connection_check_interval = '10s';
UPDATE "item_sources" SET "probed_at" = NULL
WHERE "probed_at" IS NOT NULL
  AND "id" IN (SELECT "item_source_id" FROM "keyframe_indexes" WHERE jsonb_array_length("ticks") > 0 AND ("ticks"->>0)::bigint > 0);
DELETE FROM "keyframe_indexes" WHERE jsonb_array_length("ticks") > 0 AND ("ticks"->>0)::bigint > 0;
