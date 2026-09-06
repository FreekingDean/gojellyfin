-- Modify "sources" table
ALTER TABLE "sources" DROP COLUMN IF EXISTS "path_mappings", DROP COLUMN IF EXISTS "libraries";
-- Create "library_sources" table
CREATE TABLE IF NOT EXISTS "library_sources" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "tag_filter" character varying NULL,
  "source_path" character varying NULL,
  "target_path" character varying NULL,
  "library_id" uuid NOT NULL,
  "source_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "library_sources_libraries_sources" FOREIGN KEY ("library_id") REFERENCES "libraries" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "library_sources_sources_libraries" FOREIGN KEY ("source_id") REFERENCES "sources" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "librarysource_library_id_source_id" to table: "library_sources"
CREATE UNIQUE INDEX IF NOT EXISTS "librarysource_library_id_source_id" ON "library_sources" ("library_id", "source_id");
-- Create index "librarysource_source_id" to table: "library_sources"
CREATE INDEX IF NOT EXISTS "librarysource_source_id" ON "library_sources" ("source_id");
