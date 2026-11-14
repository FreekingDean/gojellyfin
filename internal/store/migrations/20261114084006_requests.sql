-- Modify "sources" table
ALTER TABLE "sources" ADD COLUMN IF NOT EXISTS "request_profile_id" bigint NULL, ADD COLUMN IF NOT EXISTS "request_root_folder" character varying NULL, ADD COLUMN IF NOT EXISTS "request_tag" character varying NULL;
-- Create "request_submissions" table
CREATE TABLE IF NOT EXISTS "request_submissions" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "key" character varying NOT NULL, "accepted_at" timestamptz NULL, "last_error" character varying NULL, "source_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "request_submissions_sources_submissions" FOREIGN KEY ("source_id") REFERENCES "sources" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "requestsubmission_source_id_key" to table: "request_submissions"
CREATE UNIQUE INDEX IF NOT EXISTS "requestsubmission_source_id_key" ON "request_submissions" ("source_id", "key");
-- Create index "requestsubmission_key" to table: "request_submissions"
CREATE INDEX IF NOT EXISTS "requestsubmission_key" ON "request_submissions" ("key");
-- Create "requests" table
CREATE TABLE IF NOT EXISTS "requests" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "key" character varying NOT NULL, "kind" character varying NOT NULL, "tmdb_id" bigint NOT NULL, "name" character varying NOT NULL, "user_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "requests_users_requests" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "request_user_id_key" to table: "requests"
CREATE UNIQUE INDEX IF NOT EXISTS "request_user_id_key" ON "requests" ("user_id", "key");
-- Create index "request_key" to table: "requests"
CREATE INDEX IF NOT EXISTS "request_key" ON "requests" ("key");
