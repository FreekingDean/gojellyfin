-- Create "sources" table
CREATE TABLE "sources" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "name" character varying NOT NULL,
  "url" character varying NOT NULL,
  "api_key" character varying NOT NULL,
  "path_mappings" jsonb NULL,
  "libraries" jsonb NULL,
  "kind" character varying NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "sources_name_key" to table: "sources"
CREATE UNIQUE INDEX "sources_name_key" ON "sources" ("name");
-- Create index "sources_url_key" to table: "sources"
CREATE UNIQUE INDEX "sources_url_key" ON "sources" ("url");
