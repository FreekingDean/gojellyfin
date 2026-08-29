-- Modify "images" table
ALTER TABLE "images" ADD COLUMN IF NOT EXISTS "source" character varying NOT NULL DEFAULT 'Local';
-- Create "image_blobs" table
CREATE TABLE IF NOT EXISTS "image_blobs" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "key" character varying NOT NULL,
  "data" bytea NOT NULL,
  PRIMARY KEY ("id")
);
-- JPEG and PNG are already compressed, so EXTERNAL toasts the bytes without
-- spending CPU on a pass that cannot shrink them.
ALTER TABLE "image_blobs" ALTER COLUMN "data" SET STORAGE EXTERNAL;
-- Create index "imageblob_key" to table: "image_blobs"
CREATE UNIQUE INDEX IF NOT EXISTS "imageblob_key" ON "image_blobs" ("key");
