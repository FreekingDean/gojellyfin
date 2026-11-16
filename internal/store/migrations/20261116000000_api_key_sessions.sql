SET LOCAL lock_timeout = '30s';
SET LOCAL client_connection_check_interval = '10s';
ALTER TABLE "api_keys" ADD COLUMN IF NOT EXISTS "user_id" uuid NULL;
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'api_keys_users_api_keys') THEN
    ALTER TABLE "api_keys" ADD CONSTRAINT "api_keys_users_api_keys" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE;
  END IF;
END $$;
CREATE INDEX IF NOT EXISTS "apikey_user_id" ON "api_keys" ("user_id");
UPDATE "api_keys" SET "user_id" = (
  SELECT "users"."id" FROM "users"
  JOIN "user_policies" ON "user_policies"."user_policy" = "users"."id"
  WHERE "user_policies"."is_administrator" AND NOT "user_policies"."is_disabled"
  ORDER BY "users"."created_at", "users"."id"
  LIMIT 1
) WHERE "user_id" IS NULL;
ALTER TABLE "sessions" ADD COLUMN IF NOT EXISTS "api_key_id" uuid NULL;
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sessions_api_keys_sessions') THEN
    ALTER TABLE "sessions" ADD CONSTRAINT "sessions_api_keys_sessions" FOREIGN KEY ("api_key_id") REFERENCES "api_keys" ("id") ON DELETE CASCADE;
  END IF;
END $$;
CREATE INDEX IF NOT EXISTS "session_api_key_id" ON "sessions" ("api_key_id");
