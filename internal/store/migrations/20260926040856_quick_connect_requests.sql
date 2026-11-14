-- Create "quick_connect_requests" table
CREATE TABLE IF NOT EXISTS "quick_connect_requests" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "secret" character varying NOT NULL,
  "code" character varying NOT NULL,
  "device_id" character varying NOT NULL,
  "device_name" character varying NOT NULL,
  "app_name" character varying NOT NULL,
  "app_version" character varying NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "authorized_by_id" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "quick_connect_requests_users_quick_connect_requests" FOREIGN KEY ("authorized_by_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "quick_connect_requests_code_key" to table: "quick_connect_requests"
CREATE UNIQUE INDEX IF NOT EXISTS "quick_connect_requests_code_key" ON "quick_connect_requests" ("code");
-- Create index "quick_connect_requests_secret_key" to table: "quick_connect_requests"
CREATE UNIQUE INDEX IF NOT EXISTS "quick_connect_requests_secret_key" ON "quick_connect_requests" ("secret");
