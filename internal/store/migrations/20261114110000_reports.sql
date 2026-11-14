-- Create "reports" table
CREATE TABLE IF NOT EXISTS "reports" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "kind" character varying NOT NULL, "origin" character varying NOT NULL, "note" character varying NOT NULL DEFAULT '', "status" character varying NOT NULL DEFAULT 'Open', "item_id" uuid NOT NULL, "item_source_id" uuid NULL, "user_id" uuid NULL, PRIMARY KEY ("id"), CONSTRAINT "reports_items_reports" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "reports_item_sources_reports" FOREIGN KEY ("item_source_id") REFERENCES "item_sources" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT "reports_users_reports" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL);
-- Create index "report_status_created_at" to table: "reports"
CREATE INDEX IF NOT EXISTS "report_status_created_at" ON "reports" ("status", "created_at");
-- Create index "report_item_id" to table: "reports"
CREATE INDEX IF NOT EXISTS "report_item_id" ON "reports" ("item_id");
