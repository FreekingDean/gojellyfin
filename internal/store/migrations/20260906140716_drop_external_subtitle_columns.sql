-- External subtitle streams were the sidecar scan's, and the scan reads no
-- filesystem any more, so nothing writes these.
ALTER TABLE "media_streams"
  DROP COLUMN IF EXISTS "path",
  DROP COLUMN IF EXISTS "is_external",
  DROP COLUMN IF EXISTS "is_hearing_impaired";
