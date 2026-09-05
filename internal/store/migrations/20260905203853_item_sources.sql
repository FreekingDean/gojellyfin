-- A legacy path key cannot be slugified in SQL and the scan that rewrote one is
-- gone, so refuse before anything here is destructive.
DO $$ BEGIN
  IF EXISTS (
    SELECT 1 FROM "items"
    WHERE "kind" IN ('Movie', 'Series', 'Season', 'Episode')
      AND "key" !~ '^(movie|series|season|episode):'
  ) THEN
    RAISE EXCEPTION 'legacy path keys remain: scan on the previous version before migrating';
  END IF;
END $$;

-- Carry each source's root forward from the bindings that agree on one.
ALTER TABLE "sources" ADD COLUMN "root_path" character varying NULL, ADD COLUMN "local_path" character varying NULL;

UPDATE "sources" s
SET "root_path" = agreed.source_path, "local_path" = agreed.target_path
FROM (
  SELECT source_id, (array_agg(source_path))[1] AS source_path, (array_agg(target_path))[1] AS target_path
  FROM "library_sources"
  WHERE source_path <> '' AND target_path <> ''
  GROUP BY source_id
  HAVING count(DISTINCT source_path) = 1 AND count(DISTINCT target_path) = 1
) agreed
WHERE agreed.source_id = s.id;

ALTER TABLE "library_sources" DROP COLUMN "source_path", DROP COLUMN "target_path";

-- media_sources becomes item_sources: a file belongs to the downloader that reported it.
ALTER TABLE "media_sources" RENAME TO "item_sources";
ALTER TABLE "item_sources" ADD COLUMN "source_id" uuid NULL;

UPDATE "item_sources" f
SET "source_id" = sole.source_id
FROM (
  SELECT library_id, (array_agg(source_id))[1] AS source_id
  FROM "library_sources"
  GROUP BY library_id
  HAVING count(*) = 1
) sole
WHERE sole.library_id = f.library_id;

DO $$
DECLARE orphaned bigint;
BEGIN
  SELECT count(*) INTO orphaned FROM "item_sources" WHERE "source_id" IS NULL;
  IF orphaned > 0 THEN
    RAISE NOTICE 'dropping % file rows whose downloader cannot be determined; the next scan re-derives them', orphaned;
  END IF;
END $$;

DELETE FROM "item_sources" WHERE "source_id" IS NULL;

-- One file per title per downloader, and no two downloaders claiming one file.
-- coalesce, because a row comparison against a NULL probed_at yields NULL and
-- would keep both rows; the index below would then refuse to build.
DELETE FROM "item_sources" a
USING "item_sources" b
WHERE a.path = b.path
  AND (coalesce(a.probed_at, '-infinity'::timestamptz), a.id)
    < (coalesce(b.probed_at, '-infinity'::timestamptz), b.id);

-- A title with several files from one downloader predates sources entirely: the
-- filesystem walk made those rows and one Radarr reports one file per movie.
-- Keep the best-probed and let the next scan re-derive whatever still exists.
DELETE FROM "item_sources" a
USING "item_sources" b
WHERE a.item_id = b.item_id AND a.source_id = b.source_id
  AND (coalesce(a.probed_at, '-infinity'::timestamptz), a.id)
    < (coalesce(b.probed_at, '-infinity'::timestamptz), b.id);

ALTER TABLE "item_sources" ALTER COLUMN "source_id" SET NOT NULL;
ALTER TABLE "item_sources" DROP CONSTRAINT "media_sources_libraries_media_sources";
ALTER TABLE "item_sources" DROP COLUMN "library_id";
ALTER TABLE "item_sources" RENAME CONSTRAINT "media_sources_items_media_sources" TO "item_sources_items_item_sources";
ALTER TABLE "item_sources" ADD CONSTRAINT "item_sources_sources_files"
  FOREIGN KEY ("source_id") REFERENCES "sources" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;

DROP INDEX IF EXISTS "mediasource_library_id_path";
ALTER INDEX "media_sources_pkey" RENAME TO "item_sources_pkey";
CREATE UNIQUE INDEX "itemsource_item_id_source_id" ON "item_sources" ("item_id", "source_id");
CREATE UNIQUE INDEX "itemsource_path" ON "item_sources" ("path");

-- A stream belongs to a file.
ALTER TABLE "media_streams" RENAME COLUMN "source_id" TO "item_source_id";
ALTER TABLE "media_streams" RENAME CONSTRAINT "media_streams_media_sources_streams" TO "media_streams_item_sources_streams";
ALTER INDEX "mediastream_source_id_index" RENAME TO "mediastream_item_source_id_index";
