-- Modify "sources" table
ALTER TABLE "sources" ADD COLUMN IF NOT EXISTS "version" character varying NULL, ADD COLUMN IF NOT EXISTS "contacted_at" timestamptz NULL, ADD COLUMN IF NOT EXISTS "last_error" character varying NULL;
-- Create "source_paths" table
CREATE TABLE IF NOT EXISTS "source_paths" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "remote_path" character varying NOT NULL, "local_path" character varying NOT NULL, "source_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "source_paths_sources_paths" FOREIGN KEY ("source_id") REFERENCES "sources" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "source_paths_local_path_key" to table: "source_paths"
CREATE UNIQUE INDEX IF NOT EXISTS "source_paths_local_path_key" ON "source_paths" ("local_path");
-- Create index "sourcepath_source_id_remote_path" to table: "source_paths"
CREATE UNIQUE INDEX IF NOT EXISTS "sourcepath_source_id_remote_path" ON "source_paths" ("source_id", "remote_path");
-- Move each source's one root into "source_paths"
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'sources' AND column_name = 'root_path') THEN
    INSERT INTO "source_paths" ("created_at", "updated_at", "source_id", "remote_path", "local_path")
    SELECT now(), now(), "id", "root_path", "local_path" FROM "sources"
    WHERE COALESCE("root_path", '') <> '' AND COALESCE("local_path", '') <> ''
    ON CONFLICT DO NOTHING;
  END IF;
END $$;
-- Modify "sources" table
ALTER TABLE "sources" DROP COLUMN IF EXISTS "root_path", DROP COLUMN IF EXISTS "local_path";
