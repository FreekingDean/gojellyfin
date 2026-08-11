-- Create index "displaypreferences_client_user_display_preferences" to table: "display_preferences"
CREATE UNIQUE INDEX IF NOT EXISTS "displaypreferences_client_user_display_preferences" ON "display_preferences" ("client", "user_display_preferences") WHERE (item_display_preferences IS NULL);
