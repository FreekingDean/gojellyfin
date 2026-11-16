SET LOCAL lock_timeout = '30s';
SET LOCAL client_connection_check_interval = '10s';
ALTER TABLE "sessions" ADD COLUMN IF NOT EXISTS "viewing_at" timestamptz NULL, ADD COLUMN IF NOT EXISTS "now_viewing_item_id" uuid NULL;
ALTER TABLE "sessions" DROP CONSTRAINT IF EXISTS "sessions_items_viewing_sessions";
ALTER TABLE "sessions" ADD CONSTRAINT "sessions_items_viewing_sessions" FOREIGN KEY ("now_viewing_item_id") REFERENCES "items" ("id") ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS "session_now_viewing_item_id" ON "sessions" ("now_viewing_item_id");
CREATE TABLE IF NOT EXISTS "session_additional_users" ("session_id" uuid NOT NULL, "user_id" uuid NOT NULL, PRIMARY KEY ("session_id", "user_id"), CONSTRAINT "session_additional_users_session_id" FOREIGN KEY ("session_id") REFERENCES "sessions" ("id") ON DELETE CASCADE, CONSTRAINT "session_additional_users_user_id" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE);
CREATE INDEX IF NOT EXISTS "session_additional_users_user_id" ON "session_additional_users" ("user_id");
