-- An image row used to point at a storage key, which cannot be turned back
-- into the url it was downloaded from, so the rows go and a forced metadata
-- refresh writes them again.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'images' AND column_name = 'path') THEN
    DELETE FROM "images";
  END IF;
END $$;

ALTER TABLE "images"
  DROP COLUMN IF EXISTS "path",
  DROP COLUMN IF EXISTS "blur_hash",
  DROP COLUMN IF EXISTS "width",
  DROP COLUMN IF EXISTS "height",
  DROP COLUMN IF EXISTS "size",
  ADD COLUMN IF NOT EXISTS "url" character varying NOT NULL;

DROP TABLE IF EXISTS "image_blobs";
