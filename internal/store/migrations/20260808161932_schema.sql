-- Create "users" table
CREATE TABLE "users" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "name" character varying NOT NULL,
  "username" character varying NOT NULL,
  "password_hash" character varying NOT NULL,
  PRIMARY KEY ("id")
);
-- Create "user_configurations" table
CREATE TABLE "user_configurations" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "cast_receiver_id" character varying NOT NULL,
  "audio_language_preference" character varying NOT NULL,
  "play_default_audio_track" boolean NOT NULL,
  "subtitle_language_preference" character varying NOT NULL,
  "subtitle_mode" character varying NOT NULL,
  "gropued_folders" jsonb NOT NULL,
  "ordered_views" jsonb NOT NULL,
  "latest_items_excludes" jsonb NOT NULL,
  "my_media_excludes" jsonb NOT NULL,
  "display_missing_episodes" boolean NOT NULL,
  "display_collections_view" boolean NOT NULL,
  "enabled_local_password" boolean NOT NULL,
  "hide_played_in_latest" boolean NOT NULL,
  "remember_audio_selections" boolean NOT NULL,
  "enable_next_episode_auto_play" boolean NOT NULL,
  "user_configuration" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "user_configurations_users_configuration" FOREIGN KEY ("user_configuration") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL
);
