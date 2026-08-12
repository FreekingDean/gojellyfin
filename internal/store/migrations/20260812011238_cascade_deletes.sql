-- Modify "items" table
ALTER TABLE "items" DROP CONSTRAINT IF EXISTS "items_items_children", DROP CONSTRAINT IF EXISTS "items_libraries_items", ADD CONSTRAINT "items_items_children" FOREIGN KEY ("parent_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, ADD CONSTRAINT "items_libraries_items" FOREIGN KEY ("library_id") REFERENCES "libraries" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "chapters" table
ALTER TABLE "chapters" DROP CONSTRAINT IF EXISTS "chapters_items_chapters", ADD CONSTRAINT "chapters_items_chapters" FOREIGN KEY ("item_chapters") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "credits" table
ALTER TABLE "credits" DROP CONSTRAINT IF EXISTS "credits_items_credits", DROP CONSTRAINT IF EXISTS "credits_persons_credits", ADD CONSTRAINT "credits_items_credits" FOREIGN KEY ("item_credits") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, ADD CONSTRAINT "credits_persons_credits" FOREIGN KEY ("person_credits") REFERENCES "persons" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "display_preferences" table
ALTER TABLE "display_preferences" DROP CONSTRAINT IF EXISTS "display_preferences_items_display_preferences", DROP CONSTRAINT IF EXISTS "display_preferences_users_display_preferences", ADD CONSTRAINT "display_preferences_items_display_preferences" FOREIGN KEY ("item_display_preferences") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, ADD CONSTRAINT "display_preferences_users_display_preferences" FOREIGN KEY ("user_display_preferences") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "images" table
ALTER TABLE "images" DROP CONSTRAINT IF EXISTS "images_items_images", ADD CONSTRAINT "images_items_images" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "library_options" table
ALTER TABLE "library_options" DROP CONSTRAINT IF EXISTS "library_options_libraries_options", ADD CONSTRAINT "library_options_libraries_options" FOREIGN KEY ("library_options") REFERENCES "libraries" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "media_sources" table
ALTER TABLE "media_sources" DROP CONSTRAINT IF EXISTS "media_sources_items_media_sources", ADD CONSTRAINT "media_sources_items_media_sources" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "media_attachments" table
ALTER TABLE "media_attachments" DROP CONSTRAINT IF EXISTS "media_attachments_media_sources_attachments", ADD CONSTRAINT "media_attachments_media_sources_attachments" FOREIGN KEY ("media_source_attachments") REFERENCES "media_sources" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "media_segments" table
ALTER TABLE "media_segments" DROP CONSTRAINT IF EXISTS "media_segments_items_media_segments", ADD CONSTRAINT "media_segments_items_media_segments" FOREIGN KEY ("item_media_segments") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "media_streams" table
ALTER TABLE "media_streams" DROP CONSTRAINT IF EXISTS "media_streams_media_sources_streams", ADD CONSTRAINT "media_streams_media_sources_streams" FOREIGN KEY ("source_id") REFERENCES "media_sources" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "playlists" table
ALTER TABLE "playlists" DROP CONSTRAINT IF EXISTS "playlists_items_playlist", DROP CONSTRAINT IF EXISTS "playlists_users_playlists", ADD CONSTRAINT "playlists_items_playlist" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, ADD CONSTRAINT "playlists_users_playlists" FOREIGN KEY ("owner_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "playlist_entries" table
ALTER TABLE "playlist_entries" DROP CONSTRAINT IF EXISTS "playlist_entries_items_playlist_entries", DROP CONSTRAINT IF EXISTS "playlist_entries_playlists_entries", ADD CONSTRAINT "playlist_entries_items_playlist_entries" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, ADD CONSTRAINT "playlist_entries_playlists_entries" FOREIGN KEY ("playlist_id") REFERENCES "playlists" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "playlist_shares" table
ALTER TABLE "playlist_shares" DROP CONSTRAINT IF EXISTS "playlist_shares_playlists_shares", DROP CONSTRAINT IF EXISTS "playlist_shares_users_playlist_shares", ADD CONSTRAINT "playlist_shares_playlists_shares" FOREIGN KEY ("playlist_id") REFERENCES "playlists" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, ADD CONSTRAINT "playlist_shares_users_playlist_shares" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "sessions" table
ALTER TABLE "sessions" DROP CONSTRAINT IF EXISTS "sessions_devices_sessions", DROP CONSTRAINT IF EXISTS "sessions_users_sessions", ADD CONSTRAINT "sessions_devices_sessions" FOREIGN KEY ("device_sessions") REFERENCES "devices" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, ADD CONSTRAINT "sessions_users_sessions" FOREIGN KEY ("user_sessions") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "trickplays" table
ALTER TABLE "trickplays" DROP CONSTRAINT IF EXISTS "trickplays_items_trickplays", ADD CONSTRAINT "trickplays_items_trickplays" FOREIGN KEY ("item_trickplays") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "user_configurations" table
ALTER TABLE "user_configurations" DROP CONSTRAINT IF EXISTS "user_configurations_users_configuration", ADD CONSTRAINT "user_configurations_users_configuration" FOREIGN KEY ("user_configuration") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "user_item_data" table
ALTER TABLE "user_item_data" DROP CONSTRAINT IF EXISTS "user_item_data_items_user_data", DROP CONSTRAINT IF EXISTS "user_item_data_users_item_data", ADD CONSTRAINT "user_item_data_items_user_data" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, ADD CONSTRAINT "user_item_data_users_item_data" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Modify "user_policies" table
ALTER TABLE "user_policies" DROP CONSTRAINT IF EXISTS "user_policies_users_policy", ADD CONSTRAINT "user_policies_users_policy" FOREIGN KEY ("user_policy") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
