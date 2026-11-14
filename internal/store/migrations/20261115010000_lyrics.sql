-- Modify "item_sources" table
ALTER TABLE "item_sources" ADD COLUMN IF NOT EXISTS "lyrics" text NULL;
