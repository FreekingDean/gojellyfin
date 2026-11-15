SET LOCAL lock_timeout = '30s';
SET LOCAL client_connection_check_interval = '10s';
ALTER TABLE "sessions"
  ADD COLUMN IF NOT EXISTS "position_ticks" bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS "play_method" character varying NULL,
  ADD COLUMN IF NOT EXISTS "is_paused" boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS "playing_at" timestamptz NULL,
  ADD COLUMN IF NOT EXISTS "now_playing_item_id" uuid NULL;
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sessions_items_playing_sessions') THEN
    ALTER TABLE "sessions" ADD CONSTRAINT "sessions_items_playing_sessions" FOREIGN KEY ("now_playing_item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;
  END IF;
END $$;
