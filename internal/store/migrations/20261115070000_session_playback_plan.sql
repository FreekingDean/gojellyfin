SET LOCAL lock_timeout = '30s';
SET LOCAL client_connection_check_interval = '10s';
ALTER TABLE "sessions"
  ADD COLUMN IF NOT EXISTS "playback_plan" jsonb NULL,
  ADD COLUMN IF NOT EXISTS "planned_source_id" uuid NULL;
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sessions_item_sources_planning_sessions') THEN
    ALTER TABLE "sessions" ADD CONSTRAINT "sessions_item_sources_planning_sessions" FOREIGN KEY ("planned_source_id") REFERENCES "item_sources" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;
  END IF;
END $$;
CREATE INDEX IF NOT EXISTS "session_planned_source_id" ON "sessions" ("planned_source_id");
