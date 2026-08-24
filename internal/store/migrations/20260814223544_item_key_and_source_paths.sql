-- Modify "items" table
ALTER TABLE "items" ADD COLUMN "key" character varying NULL;
-- Stand the old identity in as the new one so no row is left keyless, which a
-- NOT IN sweep can never reach. The next scan re-keys every title by name.
UPDATE "items" SET "key" = "path" WHERE "key" IS NULL AND "path" IS NOT NULL;
ALTER TABLE "items" DROP COLUMN "path";
-- Create index "item_library_id_key" to table: "items"
CREATE UNIQUE INDEX "item_library_id_key" ON "items" ("library_id", "key");
-- Modify "media_sources" table
ALTER TABLE "media_sources" ADD COLUMN "date_modified" timestamptz NULL, ADD COLUMN "probed_at" timestamptz NULL, ADD COLUMN "library_id" uuid NULL;
UPDATE "media_sources" SET "library_id" = "items"."library_id" FROM "items" WHERE "media_sources"."item_id" = "items"."id" AND "media_sources"."library_id" IS NULL;
DELETE FROM "media_sources" WHERE "library_id" IS NULL;
ALTER TABLE "media_sources" ALTER COLUMN "library_id" SET NOT NULL, ADD CONSTRAINT "media_sources_libraries_media_sources" FOREIGN KEY ("library_id") REFERENCES "libraries" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Create index "mediasource_library_id_path" to table: "media_sources"
CREATE UNIQUE INDEX "mediasource_library_id_path" ON "media_sources" ("library_id", "path");
