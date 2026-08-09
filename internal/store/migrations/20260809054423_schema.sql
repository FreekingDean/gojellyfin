-- Rename a column from "enabled_local_password" to "enable_local_password"
ALTER TABLE "user_configurations" RENAME COLUMN "enabled_local_password" TO "enable_local_password";
-- Modify "user_configurations" table
ALTER TABLE "user_configurations" ALTER COLUMN "cast_receiver_id" DROP NOT NULL, ALTER COLUMN "audio_language_preference" DROP NOT NULL, ALTER COLUMN "subtitle_language_preference" DROP NOT NULL, DROP COLUMN "gropued_folders", ALTER COLUMN "ordered_views" DROP NOT NULL, ALTER COLUMN "latest_items_excludes" DROP NOT NULL, ALTER COLUMN "my_media_excludes" DROP NOT NULL, ADD COLUMN "grouped_folders" jsonb NULL, ADD COLUMN "remember_subtitle_selections" boolean NOT NULL;
-- Create "api_keys" table
CREATE TABLE "api_keys" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "access_token" character varying NOT NULL,
  "app_name" character varying NOT NULL,
  "revoked_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "api_keys_access_token_key" to table: "api_keys"
CREATE UNIQUE INDEX "api_keys_access_token_key" ON "api_keys" ("access_token");
-- Create "libraries" table
CREATE TABLE "libraries" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "name" character varying NOT NULL,
  "collection_type" character varying NOT NULL,
  "locations" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
-- Create "listings_providers" table
CREATE TABLE "listings_providers" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "kind" character varying NOT NULL,
  "username" character varying NULL,
  "password" character varying NULL,
  "listings_id" character varying NULL,
  "zip_code" character varying NULL,
  "country" character varying NULL,
  "path" character varying NULL,
  "movie_prefix" character varying NULL,
  "preferred_language" character varying NULL,
  "user_agent" character varying NULL,
  "enable_all_tuners" boolean NOT NULL,
  "enabled_tuners" jsonb NULL,
  "news_categories" jsonb NULL,
  "sports_categories" jsonb NULL,
  "kids_categories" jsonb NULL,
  "movie_categories" jsonb NULL,
  "channel_mappings" jsonb NULL,
  PRIMARY KEY ("id")
);
-- Create "plugins" table
CREATE TABLE "plugins" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "name" character varying NOT NULL,
  "version" character varying NOT NULL,
  "description" text NULL,
  "configuration_file_name" character varying NULL,
  "status" character varying NOT NULL,
  "can_uninstall" boolean NOT NULL,
  "has_image" boolean NOT NULL,
  PRIMARY KEY ("id")
);
-- Create "tuner_hosts" table
CREATE TABLE "tuner_hosts" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "kind" character varying NOT NULL,
  "url" character varying NOT NULL,
  "device_id" character varying NULL,
  "friendly_name" character varying NULL,
  "source" character varying NULL,
  "user_agent" character varying NULL,
  "tuner_count" integer NOT NULL,
  "fallback_max_streaming_bitrate" integer NOT NULL,
  "import_favorites_only" boolean NOT NULL,
  "allow_hw_transcoding" boolean NOT NULL,
  "allow_fmp4_transcoding_container" boolean NOT NULL,
  "allow_stream_sharing" boolean NOT NULL,
  "enable_stream_looping" boolean NOT NULL,
  "ignore_dts" boolean NOT NULL,
  PRIMARY KEY ("id")
);
-- Create "items" table
CREATE TABLE "items" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "kind" character varying NOT NULL,
  "media_type" character varying NOT NULL,
  "location_type" character varying NOT NULL,
  "extra_type" character varying NULL,
  "video_type" character varying NULL,
  "iso_type" character varying NULL,
  "video_3d_format" character varying NULL,
  "name" character varying NOT NULL,
  "original_title" character varying NULL,
  "sort_name" character varying NULL,
  "forced_sort_name" boolean NOT NULL,
  "path" character varying NULL,
  "container" character varying NULL,
  "overview" text NULL,
  "is_folder" boolean NOT NULL,
  "is_placeholder" boolean NOT NULL,
  "lock_data" boolean NOT NULL,
  "has_lyrics" boolean NOT NULL,
  "has_subtitles" boolean NOT NULL,
  "enable_media_source_display" boolean NOT NULL,
  "premiere_date" timestamptz NULL,
  "end_date" timestamptz NULL,
  "last_media_added_at" timestamptz NULL,
  "production_year" integer NULL,
  "official_rating" character varying NULL,
  "custom_rating" character varying NULL,
  "critic_rating" double precision NULL,
  "community_rating" double precision NULL,
  "run_time_ticks" bigint NULL,
  "index_number" integer NULL,
  "index_number_end" integer NULL,
  "parent_index_number" integer NULL,
  "airs_before_season_number" integer NULL,
  "airs_after_season_number" integer NULL,
  "airs_before_episode_number" integer NULL,
  "status" character varying NULL,
  "air_time" character varying NULL,
  "display_order" character varying NULL,
  "air_days" jsonb NULL,
  "aspect_ratio" character varying NULL,
  "width" integer NULL,
  "height" integer NULL,
  "normalization_gain" double precision NULL,
  "preferred_metadata_language" character varying NULL,
  "preferred_metadata_country_code" character varying NULL,
  "provider_ids" jsonb NULL,
  "tags" jsonb NULL,
  "taglines" jsonb NULL,
  "production_locations" jsonb NULL,
  "locked_fields" jsonb NULL,
  "external_urls" jsonb NULL,
  "item_children" uuid NULL,
  "library_items" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "items_items_children" FOREIGN KEY ("item_children") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "items_libraries_items" FOREIGN KEY ("library_items") REFERENCES "libraries" ("id") ON UPDATE NO ACTION ON DELETE SET NULL
);
-- Create index "item_kind_sort_name" to table: "items"
CREATE INDEX "item_kind_sort_name" ON "items" ("kind", "sort_name");
-- Create index "item_path" to table: "items"
CREATE INDEX "item_path" ON "items" ("path");
-- Create "activity_log_entries" table
CREATE TABLE "activity_log_entries" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "name" character varying NOT NULL,
  "kind" character varying NOT NULL,
  "overview" text NULL,
  "short_overview" text NULL,
  "severity" character varying NOT NULL,
  "item_activity_log_entries" uuid NULL,
  "user_activity_log_entries" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "activity_log_entries_items_activity_log_entries" FOREIGN KEY ("item_activity_log_entries") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "activity_log_entries_users_activity_log_entries" FOREIGN KEY ("user_activity_log_entries") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL
);
-- Create "chapters" table
CREATE TABLE "chapters" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "name" character varying NULL,
  "start_position_ticks" bigint NOT NULL,
  "image_path" character varying NULL,
  "image_modified_at" timestamptz NULL,
  "item_chapters" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "chapters_items_chapters" FOREIGN KEY ("item_chapters") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create "persons" table
CREATE TABLE "persons" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "name" character varying NOT NULL,
  "overview" text NULL,
  "birth_date" timestamptz NULL,
  "death_date" timestamptz NULL,
  "provider_ids" jsonb NULL,
  PRIMARY KEY ("id")
);
-- Create index "persons_name_key" to table: "persons"
CREATE UNIQUE INDEX "persons_name_key" ON "persons" ("name");
-- Create "credits" table
CREATE TABLE "credits" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "kind" character varying NOT NULL,
  "role" character varying NULL,
  "sort_order" integer NULL,
  "item_credits" uuid NOT NULL,
  "person_credits" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "credits_items_credits" FOREIGN KEY ("item_credits") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "credits_persons_credits" FOREIGN KEY ("person_credits") REFERENCES "persons" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "credit_kind_role_item_credits_person_credits" to table: "credits"
CREATE UNIQUE INDEX "credit_kind_role_item_credits_person_credits" ON "credits" ("kind", "role", "item_credits", "person_credits");
-- Create "display_preferences" table
CREATE TABLE "display_preferences" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "client" character varying NOT NULL,
  "view_type" character varying NULL,
  "sort_by" character varying NULL,
  "index_by" character varying NULL,
  "sort_order" character varying NOT NULL,
  "scroll_direction" character varying NOT NULL,
  "remember_indexing" boolean NOT NULL,
  "remember_sorting" boolean NOT NULL,
  "show_backdrop" boolean NOT NULL,
  "show_sidebar" boolean NOT NULL,
  "primary_image_height" integer NOT NULL,
  "primary_image_width" integer NOT NULL,
  "custom_prefs" jsonb NULL,
  "item_display_preferences" uuid NULL,
  "user_display_preferences" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "display_preferences_items_display_preferences" FOREIGN KEY ("item_display_preferences") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "display_preferences_users_display_preferences" FOREIGN KEY ("user_display_preferences") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "displaypreferences_client_user_display_preferences_item_display" to table: "display_preferences"
CREATE UNIQUE INDEX "displaypreferences_client_user_display_preferences_item_display" ON "display_preferences" ("client", "user_display_preferences", "item_display_preferences");
-- Create "images" table
CREATE TABLE "images" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "kind" character varying NOT NULL,
  "index" integer NOT NULL,
  "path" character varying NOT NULL,
  "blur_hash" character varying NULL,
  "width" integer NULL,
  "height" integer NULL,
  "size" bigint NULL,
  "item_images" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "images_items_images" FOREIGN KEY ("item_images") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "image_kind_index_item_images" to table: "images"
CREATE UNIQUE INDEX "image_kind_index_item_images" ON "images" ("kind", "index", "item_images");
-- Create "genres" table
CREATE TABLE "genres" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "name" character varying NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "genres_name_key" to table: "genres"
CREATE UNIQUE INDEX "genres_name_key" ON "genres" ("name");
-- Create "item_genres" table
CREATE TABLE "item_genres" (
  "item_id" uuid NOT NULL,
  "genre_id" uuid NOT NULL,
  PRIMARY KEY ("item_id", "genre_id"),
  CONSTRAINT "item_genres_genre_id" FOREIGN KEY ("genre_id") REFERENCES "genres" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "item_genres_item_id" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "studios" table
CREATE TABLE "studios" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "name" character varying NOT NULL,
  "provider_ids" jsonb NULL,
  PRIMARY KEY ("id")
);
-- Create index "studios_name_key" to table: "studios"
CREATE UNIQUE INDEX "studios_name_key" ON "studios" ("name");
-- Create "item_studios" table
CREATE TABLE "item_studios" (
  "item_id" uuid NOT NULL,
  "studio_id" uuid NOT NULL,
  PRIMARY KEY ("item_id", "studio_id"),
  CONSTRAINT "item_studios_item_id" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "item_studios_studio_id" FOREIGN KEY ("studio_id") REFERENCES "studios" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "library_options" table
CREATE TABLE "library_options" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "enabled" boolean NOT NULL,
  "enable_photos" boolean NOT NULL,
  "enable_realtime_monitor" boolean NOT NULL,
  "enable_lufs_scan" boolean NOT NULL,
  "enable_chapter_image_extraction" boolean NOT NULL,
  "extract_chapter_images_during_library_scan" boolean NOT NULL,
  "enable_trickplay_image_extraction" boolean NOT NULL,
  "extract_trickplay_images_during_library_scan" boolean NOT NULL,
  "save_local_metadata" boolean NOT NULL,
  "enable_internet_providers" boolean NOT NULL,
  "enable_automatic_series_grouping" boolean NOT NULL,
  "enable_embedded_titles" boolean NOT NULL,
  "enable_embedded_extras_titles" boolean NOT NULL,
  "enable_embedded_episode_infos" boolean NOT NULL,
  "skip_subtitles_if_embedded_subtitles_present" boolean NOT NULL,
  "skip_subtitles_if_audio_track_matches" boolean NOT NULL,
  "require_perfect_subtitle_match" boolean NOT NULL,
  "save_subtitles_with_media" boolean NOT NULL,
  "save_lyrics_with_media" boolean NOT NULL,
  "save_trickplay_with_media" boolean NOT NULL,
  "prefer_nonstandard_artists_tag" boolean NOT NULL,
  "use_custom_tag_delimiters" boolean NOT NULL,
  "automatically_add_to_collection" boolean NOT NULL,
  "automatic_refresh_interval_days" integer NOT NULL,
  "preferred_metadata_language" character varying NOT NULL,
  "metadata_country_code" character varying NOT NULL,
  "season_zero_display_name" character varying NOT NULL,
  "allow_embedded_subtitles" character varying NOT NULL,
  "metadata_savers" jsonb NULL,
  "disabled_local_metadata_readers" jsonb NULL,
  "local_metadata_reader_order" jsonb NULL,
  "disabled_subtitle_fetchers" jsonb NULL,
  "subtitle_fetcher_order" jsonb NULL,
  "disabled_media_segment_providers" jsonb NULL,
  "media_segment_provider_order" jsonb NULL,
  "subtitle_download_languages" jsonb NULL,
  "disabled_lyric_fetchers" jsonb NULL,
  "lyric_fetcher_order" jsonb NULL,
  "custom_tag_delimiters" jsonb NULL,
  "delimiter_whitelist" jsonb NULL,
  "type_options" jsonb NULL,
  "library_options" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "library_options_libraries_options" FOREIGN KEY ("library_options") REFERENCES "libraries" ("id") ON UPDATE NO ACTION ON DELETE SET NULL
);
-- Create index "library_options_library_options_key" to table: "library_options"
CREATE UNIQUE INDEX "library_options_library_options_key" ON "library_options" ("library_options");
-- Create "media_sources" table
CREATE TABLE "media_sources" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "protocol" character varying NOT NULL,
  "encoder_protocol" character varying NULL,
  "kind" character varying NOT NULL,
  "timestamp" character varying NULL,
  "video_type" character varying NULL,
  "iso_type" character varying NULL,
  "video_3d_format" character varying NULL,
  "name" character varying NOT NULL,
  "path" character varying NOT NULL,
  "encoder_path" character varying NULL,
  "container" character varying NULL,
  "size" bigint NULL,
  "run_time_ticks" bigint NULL,
  "bitrate" integer NULL,
  "is_remote" boolean NOT NULL,
  "is_infinite_stream" boolean NOT NULL,
  "supports_transcoding" boolean NOT NULL,
  "supports_direct_stream" boolean NOT NULL,
  "supports_direct_play" boolean NOT NULL,
  "supports_probing" boolean NOT NULL,
  "read_at_native_framerate" boolean NOT NULL,
  "ignore_dts" boolean NOT NULL,
  "ignore_index" boolean NOT NULL,
  "gen_pts_input" boolean NOT NULL,
  "requires_looping" boolean NOT NULL,
  "has_segments" boolean NOT NULL,
  "default_audio_stream_index" integer NULL,
  "default_subtitle_stream_index" integer NULL,
  "formats" jsonb NULL,
  "item_media_sources" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "media_sources_items_media_sources" FOREIGN KEY ("item_media_sources") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create "media_attachments" table
CREATE TABLE "media_attachments" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "index" integer NOT NULL,
  "codec" character varying NULL,
  "codec_tag" character varying NULL,
  "comment" character varying NULL,
  "file_name" character varying NULL,
  "mime_type" character varying NULL,
  "media_source_attachments" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "media_attachments_media_sources_attachments" FOREIGN KEY ("media_source_attachments") REFERENCES "media_sources" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "mediaattachment_index_media_source_attachments" to table: "media_attachments"
CREATE UNIQUE INDEX "mediaattachment_index_media_source_attachments" ON "media_attachments" ("index", "media_source_attachments");
-- Create "media_segments" table
CREATE TABLE "media_segments" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "kind" character varying NOT NULL,
  "start_ticks" bigint NOT NULL,
  "end_ticks" bigint NOT NULL,
  "item_media_segments" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "media_segments_items_media_segments" FOREIGN KEY ("item_media_segments") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create "media_streams" table
CREATE TABLE "media_streams" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "kind" character varying NOT NULL,
  "video_range" character varying NULL,
  "video_range_type" character varying NULL,
  "audio_spatial_format" character varying NULL,
  "index" integer NOT NULL,
  "codec" character varying NULL,
  "codec_tag" character varying NULL,
  "profile" character varying NULL,
  "language" character varying NULL,
  "title" character varying NULL,
  "comment" character varying NULL,
  "path" character varying NULL,
  "pixel_format" character varying NULL,
  "aspect_ratio" character varying NULL,
  "channel_layout" character varying NULL,
  "time_base" character varying NULL,
  "nal_length_size" character varying NULL,
  "video_dovi_title" character varying NULL,
  "color_range" character varying NULL,
  "color_space" character varying NULL,
  "color_transfer" character varying NULL,
  "color_primaries" character varying NULL,
  "dv_version_major" integer NULL,
  "dv_version_minor" integer NULL,
  "dv_profile" integer NULL,
  "dv_level" integer NULL,
  "rpu_present_flag" integer NULL,
  "el_present_flag" integer NULL,
  "bl_present_flag" integer NULL,
  "dv_bl_signal_compatibility_id" integer NULL,
  "bit_rate" integer NULL,
  "bit_depth" integer NULL,
  "ref_frames" integer NULL,
  "packet_length" integer NULL,
  "channels" integer NULL,
  "sample_rate" integer NULL,
  "width" integer NULL,
  "height" integer NULL,
  "rotation" integer NULL,
  "score" integer NULL,
  "level" double precision NULL,
  "average_frame_rate" double precision NULL,
  "real_frame_rate" double precision NULL,
  "reference_frame_rate" double precision NULL,
  "is_default" boolean NOT NULL,
  "is_forced" boolean NOT NULL,
  "is_external" boolean NOT NULL,
  "is_interlaced" boolean NOT NULL,
  "is_anamorphic" boolean NOT NULL,
  "is_avc" boolean NOT NULL,
  "is_hearing_impaired" boolean NOT NULL,
  "media_source_streams" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "media_streams_media_sources_streams" FOREIGN KEY ("media_source_streams") REFERENCES "media_sources" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "mediastream_index_media_source_streams" to table: "media_streams"
CREATE UNIQUE INDEX "mediastream_index_media_source_streams" ON "media_streams" ("index", "media_source_streams");
-- Create "playlists" table
CREATE TABLE "playlists" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "open_access" boolean NOT NULL,
  "item_playlist" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "playlists_items_playlist" FOREIGN KEY ("item_playlist") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "playlists_item_playlist_key" to table: "playlists"
CREATE UNIQUE INDEX "playlists_item_playlist_key" ON "playlists" ("item_playlist");
-- Create "playlist_entries" table
CREATE TABLE "playlist_entries" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "sort_order" integer NOT NULL,
  "item_playlist_entries" uuid NOT NULL,
  "playlist_entries" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "playlist_entries_items_playlist_entries" FOREIGN KEY ("item_playlist_entries") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "playlist_entries_playlists_entries" FOREIGN KEY ("playlist_entries") REFERENCES "playlists" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "playlistentry_sort_order_playlist_entries" to table: "playlist_entries"
CREATE INDEX "playlistentry_sort_order_playlist_entries" ON "playlist_entries" ("sort_order", "playlist_entries");
-- Create "playlist_shares" table
CREATE TABLE "playlist_shares" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "can_edit" boolean NOT NULL,
  "playlist_shares" uuid NOT NULL,
  "user_playlist_shares" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "playlist_shares_playlists_shares" FOREIGN KEY ("playlist_shares") REFERENCES "playlists" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "playlist_shares_users_playlist_shares" FOREIGN KEY ("user_playlist_shares") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "playlistshare_playlist_shares_user_playlist_shares" to table: "playlist_shares"
CREATE UNIQUE INDEX "playlistshare_playlist_shares_user_playlist_shares" ON "playlist_shares" ("playlist_shares", "user_playlist_shares");
-- Create "devices" table
CREATE TABLE "devices" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "client_id" character varying NOT NULL,
  "name" character varying NOT NULL,
  "custom_name" character varying NULL,
  "app_name" character varying NOT NULL,
  "app_version" character varying NOT NULL,
  "icon_url" character varying NULL,
  "playable_media_types" jsonb NULL,
  "supported_commands" jsonb NULL,
  "profile" jsonb NULL,
  "supports_media_control" boolean NOT NULL,
  "supports_persistent_identifier" boolean NOT NULL,
  "last_activity_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "devices_client_id_key" to table: "devices"
CREATE UNIQUE INDEX "devices_client_id_key" ON "devices" ("client_id");
-- Create "sessions" table
CREATE TABLE "sessions" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "access_token" character varying NOT NULL,
  "remote_endpoint" character varying NULL,
  "last_activity_at" timestamptz NULL,
  "revoked_at" timestamptz NULL,
  "device_sessions" uuid NOT NULL,
  "user_sessions" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "sessions_devices_sessions" FOREIGN KEY ("device_sessions") REFERENCES "devices" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "sessions_users_sessions" FOREIGN KEY ("user_sessions") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "sessions_access_token_key" to table: "sessions"
CREATE UNIQUE INDEX "sessions_access_token_key" ON "sessions" ("access_token");
-- Create "series_timers" table
CREATE TABLE "series_timers" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "name" character varying NOT NULL,
  "overview" text NULL,
  "service_name" character varying NULL,
  "external_id" character varying NULL,
  "channel_id" character varying NULL,
  "external_channel_id" character varying NULL,
  "program_id" character varying NULL,
  "external_program_id" character varying NULL,
  "start_date" timestamptz NOT NULL,
  "end_date" timestamptz NOT NULL,
  "priority" integer NOT NULL,
  "pre_padding_seconds" integer NOT NULL,
  "post_padding_seconds" integer NOT NULL,
  "is_pre_padding_required" boolean NOT NULL,
  "is_post_padding_required" boolean NOT NULL,
  "keep_until" character varying NOT NULL,
  "keep_up_to" integer NOT NULL,
  "record_any_time" boolean NOT NULL,
  "record_any_channel" boolean NOT NULL,
  "record_new_only" boolean NOT NULL,
  "skip_episodes_in_library" boolean NOT NULL,
  "days" jsonb NULL,
  "day_pattern" character varying NULL,
  PRIMARY KEY ("id")
);
-- Create "timers" table
CREATE TABLE "timers" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "name" character varying NOT NULL,
  "overview" text NULL,
  "service_name" character varying NULL,
  "external_id" character varying NULL,
  "channel_id" character varying NULL,
  "external_channel_id" character varying NULL,
  "program_id" character varying NULL,
  "external_program_id" character varying NULL,
  "external_series_timer_id" character varying NULL,
  "start_date" timestamptz NOT NULL,
  "end_date" timestamptz NOT NULL,
  "run_time_ticks" bigint NULL,
  "priority" integer NOT NULL,
  "pre_padding_seconds" integer NOT NULL,
  "post_padding_seconds" integer NOT NULL,
  "is_pre_padding_required" boolean NOT NULL,
  "is_post_padding_required" boolean NOT NULL,
  "keep_until" character varying NOT NULL,
  "status" character varying NOT NULL,
  "series_timer_timers" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "timers_series_timers_timers" FOREIGN KEY ("series_timer_timers") REFERENCES "series_timers" ("id") ON UPDATE NO ACTION ON DELETE SET NULL
);
-- Create "trickplays" table
CREATE TABLE "trickplays" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "width" integer NOT NULL,
  "height" integer NOT NULL,
  "tile_width" integer NOT NULL,
  "tile_height" integer NOT NULL,
  "thumbnail_count" integer NOT NULL,
  "interval" integer NOT NULL,
  "bandwidth" integer NOT NULL,
  "item_trickplays" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "trickplays_items_trickplays" FOREIGN KEY ("item_trickplays") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create "user_item_data" table
CREATE TABLE "user_item_data" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "played" boolean NOT NULL,
  "is_favorite" boolean NOT NULL,
  "play_count" integer NOT NULL,
  "playback_position_ticks" bigint NOT NULL,
  "rating" double precision NULL,
  "likes" boolean NULL,
  "last_played_at" timestamptz NULL,
  "item_user_data" uuid NOT NULL,
  "user_item_data" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "user_item_data_items_user_data" FOREIGN KEY ("item_user_data") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "user_item_data_users_item_data" FOREIGN KEY ("user_item_data") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "useritemdata_user_item_data_item_user_data" to table: "user_item_data"
CREATE UNIQUE INDEX "useritemdata_user_item_data_item_user_data" ON "user_item_data" ("user_item_data", "item_user_data");
-- Create "user_policies" table
CREATE TABLE "user_policies" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "is_administrator" boolean NOT NULL,
  "is_hidden" boolean NOT NULL,
  "is_disabled" boolean NOT NULL,
  "enable_collection_management" boolean NOT NULL DEFAULT false,
  "enable_subtitle_management" boolean NOT NULL DEFAULT false,
  "enable_lyric_management" boolean NOT NULL DEFAULT false,
  "enable_user_preference_access" boolean NOT NULL,
  "enable_remote_control_of_other_users" boolean NOT NULL,
  "enable_shared_device_control" boolean NOT NULL,
  "enable_remote_access" boolean NOT NULL,
  "enable_live_tv_management" boolean NOT NULL,
  "enable_live_tv_access" boolean NOT NULL,
  "enable_media_playback" boolean NOT NULL,
  "enable_audio_playback_transcoding" boolean NOT NULL,
  "enable_video_playback_transcoding" boolean NOT NULL,
  "enable_playback_remuxing" boolean NOT NULL,
  "force_remote_source_transcoding" boolean NOT NULL,
  "enable_content_deletion" boolean NOT NULL,
  "enable_content_downloading" boolean NOT NULL,
  "enable_sync_transcoding" boolean NOT NULL,
  "enable_media_conversion" boolean NOT NULL,
  "enable_public_sharing" boolean NOT NULL,
  "enable_all_devices" boolean NOT NULL,
  "enable_all_channels" boolean NOT NULL,
  "enable_all_folders" boolean NOT NULL,
  "max_parental_rating" character varying NULL,
  "max_parental_sub_rating" character varying NULL,
  "invalid_login_attempt_count" integer NOT NULL,
  "login_attempts_before_lockout" integer NOT NULL,
  "max_active_sessions" integer NOT NULL,
  "remote_client_bitrate_limit" integer NOT NULL,
  "allowed_tags" jsonb NULL,
  "blocked_tags" jsonb NULL,
  "access_schedules" jsonb NULL,
  "enable_content_deletion_from_folders" jsonb NULL,
  "enabled_devices" jsonb NULL,
  "enabled_channels" jsonb NULL,
  "enabled_folders" jsonb NULL,
  "blocked_media_folders" jsonb NULL,
  "blocked_channels" jsonb NULL,
  "block_unrated_items" jsonb NULL,
  "authentication_provider_id" character varying NOT NULL,
  "password_reset_provider_id" character varying NOT NULL,
  "sync_play_access" character varying NOT NULL,
  "user_policy" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "user_policies_users_policy" FOREIGN KEY ("user_policy") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL
);
