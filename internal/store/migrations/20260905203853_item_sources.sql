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
ALTER TABLE "sources" ADD COLUMN IF NOT EXISTS "root_path" character varying NULL, ADD COLUMN IF NOT EXISTS "local_path" character varying NULL;

DO $$ BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = current_schema() AND table_name = 'library_sources' AND column_name = 'source_path'
  ) THEN
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
  END IF;
END $$;

ALTER TABLE "library_sources" DROP COLUMN IF EXISTS "source_path", DROP COLUMN IF EXISTS "target_path";

-- media_sources becomes item_sources: a file belongs to the downloader that reported it.
DO $$ BEGIN
  IF to_regclass('"media_sources"') IS NOT NULL AND to_regclass('"item_sources"') IS NULL THEN
    ALTER TABLE "media_sources" RENAME TO "item_sources";
  END IF;
END $$;

ALTER TABLE "item_sources" ADD COLUMN IF NOT EXISTS "source_id" uuid NULL;

DO $$ BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = current_schema() AND table_name = 'item_sources' AND column_name = 'library_id'
  ) THEN
    UPDATE "item_sources" f
    SET "source_id" = sole.source_id
    FROM (
      SELECT library_id, (array_agg(source_id))[1] AS source_id
      FROM "library_sources"
      GROUP BY library_id
      HAVING count(*) = 1
    ) sole
    WHERE sole.library_id = f.library_id;
  END IF;
END $$;

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
ALTER TABLE "item_sources" DROP CONSTRAINT IF EXISTS "media_sources_libraries_media_sources";
ALTER TABLE "item_sources" DROP COLUMN IF EXISTS "library_id";

DO $$ BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = '"item_sources"'::regclass AND conname = 'media_sources_items_media_sources'
  ) THEN
    ALTER TABLE "item_sources" RENAME CONSTRAINT "media_sources_items_media_sources" TO "item_sources_items_item_sources";
  END IF;
END $$;

DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = '"item_sources"'::regclass AND conname = 'item_sources_sources_files'
  ) THEN
    ALTER TABLE "item_sources" ADD CONSTRAINT "item_sources_sources_files"
      FOREIGN KEY ("source_id") REFERENCES "sources" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
  END IF;
END $$;

DROP INDEX IF EXISTS "mediasource_library_id_path";

DO $$ BEGIN
  IF to_regclass('"media_sources_pkey"') IS NOT NULL THEN
    ALTER INDEX "media_sources_pkey" RENAME TO "item_sources_pkey";
  END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS "itemsource_item_id_source_id" ON "item_sources" ("item_id", "source_id");
CREATE UNIQUE INDEX IF NOT EXISTS "itemsource_path" ON "item_sources" ("path");

-- A stream belongs to a file.
DO $$ BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = current_schema() AND table_name = 'media_streams' AND column_name = 'source_id'
  ) THEN
    ALTER TABLE "media_streams" RENAME COLUMN "source_id" TO "item_source_id";
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = '"media_streams"'::regclass AND conname = 'media_streams_media_sources_streams'
  ) THEN
    ALTER TABLE "media_streams" RENAME CONSTRAINT "media_streams_media_sources_streams" TO "media_streams_item_sources_streams";
  END IF;
END $$;

DO $$ BEGIN
  IF to_regclass('"mediastream_source_id_index"') IS NOT NULL THEN
    ALTER INDEX "mediastream_source_id_index" RENAME TO "mediastream_item_source_id_index";
  END IF;
END $$;
