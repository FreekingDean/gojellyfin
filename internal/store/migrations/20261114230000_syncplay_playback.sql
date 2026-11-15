-- Modify "sync_play_group_members" table
ALTER TABLE "sync_play_group_members" ADD COLUMN "ping" bigint NOT NULL DEFAULT 500, ADD COLUMN "is_buffering" boolean NOT NULL DEFAULT false, ADD COLUMN "ignore_wait" boolean NOT NULL DEFAULT false;
-- Modify "sync_play_groups" table
ALTER TABLE "sync_play_groups" ADD COLUMN "state" character varying NOT NULL DEFAULT 'Idle', ADD COLUMN "position_ticks" bigint NOT NULL DEFAULT 0, ADD COLUMN "run_time_ticks" bigint NOT NULL DEFAULT 0, ADD COLUMN "last_activity" timestamptz NOT NULL DEFAULT now(), ADD COLUMN "playing_index" bigint NOT NULL DEFAULT -1, ADD COLUMN "shuffle_mode" character varying NOT NULL DEFAULT 'Sorted', ADD COLUMN "repeat_mode" character varying NOT NULL DEFAULT 'RepeatNone', ADD COLUMN "queue_changed_at" timestamptz NOT NULL DEFAULT now(), ADD COLUMN "resume_playing" boolean NOT NULL DEFAULT false, ADD COLUMN "waiting_from" character varying NULL, ADD COLUMN "position_jumped" boolean NOT NULL DEFAULT false, ADD COLUMN "ignore_buffering" boolean NOT NULL DEFAULT false, ADD COLUMN "wait_deadline" timestamptz NULL;
ALTER TABLE "sync_play_groups" ALTER COLUMN "last_activity" DROP DEFAULT, ALTER COLUMN "queue_changed_at" DROP DEFAULT;
-- Create index "syncplaygroup_wait_deadline" to table: "sync_play_groups"
CREATE INDEX "syncplaygroup_wait_deadline" ON "sync_play_groups" ("wait_deadline");
-- Create "sync_play_queue_entries" table
CREATE TABLE "sync_play_queue_entries" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "sorted_index" bigint NOT NULL,
  "shuffled_index" bigint NULL,
  "item_id" uuid NOT NULL,
  "group_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "sync_play_queue_entries_items_sync_play_entries" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "sync_play_queue_entries_sync_play_groups_queue" FOREIGN KEY ("group_id") REFERENCES "sync_play_groups" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "syncplayqueueentry_group_id_sorted_index" to table: "sync_play_queue_entries"
CREATE INDEX "syncplayqueueentry_group_id_sorted_index" ON "sync_play_queue_entries" ("group_id", "sorted_index");
-- Create index "syncplayqueueentry_item_id" to table: "sync_play_queue_entries"
CREATE INDEX "syncplayqueueentry_item_id" ON "sync_play_queue_entries" ("item_id");
