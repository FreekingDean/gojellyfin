-- Modify "display_preferences" table
ALTER TABLE "display_preferences" ADD COLUMN IF NOT EXISTS "reference_id" character varying NULL;
-- The id the client sends is opaque and was never an item, so stand the old item
-- in as the new key and give the rows that had none the only other id jellyfin-web
-- sends, which is what a null item meant.
UPDATE "display_preferences" SET "reference_id" = COALESCE("item_display_preferences"::text, 'usersettings') WHERE "reference_id" IS NULL;
DROP INDEX IF EXISTS "displaypreferences_client_user_display_preferences_item_display";
DROP INDEX IF EXISTS "displaypreferences_client_user_display_preferences";
ALTER TABLE "display_preferences" DROP COLUMN IF EXISTS "item_display_preferences";
ALTER TABLE "display_preferences" RENAME COLUMN "user_display_preferences" TO "user_id";
ALTER TABLE "display_preferences" ALTER COLUMN "reference_id" SET NOT NULL;
-- Create index "displaypreferences_user_id_reference_id_client" to table: "display_preferences"
CREATE UNIQUE INDEX IF NOT EXISTS "displaypreferences_user_id_reference_id_client" ON "display_preferences" ("user_id", "reference_id", "client");
