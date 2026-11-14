-- Nothing writes the kinds dropped from the enum, and every source-written
-- item carries a key, so a row breaking either rule is left from an older
-- build. Refuse rather than delete watch state hanging off it.
DO $$ BEGIN
  IF EXISTS (
    SELECT 1 FROM "items"
    WHERE "kind" NOT IN ('Movie', 'Series', 'Season', 'Episode', 'Playlist', 'Audio')
  ) THEN
    RAISE EXCEPTION 'items of a kind nothing writes remain: delete them before migrating';
  END IF;
  IF EXISTS (
    SELECT 1 FROM "items" WHERE "kind" <> 'Playlist' AND "key" IS NULL
  ) THEN
    RAISE EXCEPTION 'items without a key remain: rescan on the previous version before migrating';
  END IF;
END $$;

-- Modify "item_sources" table
ALTER TABLE "item_sources" ADD COLUMN IF NOT EXISTS "external_id" bigint NULL, ADD COLUMN IF NOT EXISTS "external_file_id" bigint NULL;
-- Create index "itemsource_source_id_external_file_id" to table: "item_sources"
CREATE INDEX IF NOT EXISTS "itemsource_source_id_external_file_id" ON "item_sources" ("source_id", "external_file_id");
-- Modify "items" table
ALTER TABLE "items" DROP CONSTRAINT IF EXISTS "item_key_unless_playlist", DROP CONSTRAINT IF EXISTS "item_kind_known";
ALTER TABLE "items" ADD CONSTRAINT "item_key_unless_playlist" CHECK (((kind)::text = 'Playlist'::text) OR (key IS NOT NULL)), ADD CONSTRAINT "item_kind_known" CHECK ((kind)::text = ANY ((ARRAY['Movie'::character varying, 'Series'::character varying, 'Season'::character varying, 'Episode'::character varying, 'Playlist'::character varying, 'Audio'::character varying])::text[]));
-- Modify "library_sources" table
ALTER TABLE "library_sources" ADD COLUMN IF NOT EXISTS "last_synced_at" timestamptz NULL, ADD COLUMN IF NOT EXISTS "last_error" character varying NULL;
