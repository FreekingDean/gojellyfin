-- A playlist's media type is the one real value the column held; every other
-- item's was a function of its kind.
ALTER TABLE "playlists" ADD COLUMN IF NOT EXISTS "media_type" character varying NOT NULL DEFAULT 'Unknown';

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'items' AND column_name = 'media_type') THEN
    UPDATE "playlists" p SET "media_type" = i.media_type
    FROM "items" i WHERE i.id = p.item_id AND i.media_type IS NOT NULL;
  END IF;
END $$;

ALTER TABLE "items" DROP COLUMN IF EXISTS "media_type", DROP COLUMN IF EXISTS "is_folder";
