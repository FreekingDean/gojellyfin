-- Modify "libraries" table
ALTER TABLE "libraries" ADD COLUMN IF NOT EXISTS "image_tag" character varying NOT NULL DEFAULT '';
