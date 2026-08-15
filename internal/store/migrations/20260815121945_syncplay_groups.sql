-- Create "sync_play_groups" table
CREATE TABLE "sync_play_groups" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "name" character varying NOT NULL,
  "state" character varying NOT NULL DEFAULT 'Idle',
  PRIMARY KEY ("id")
);
-- Create "sync_play_group_members" table
CREATE TABLE "sync_play_group_members" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "session_id" uuid NOT NULL,
  "group_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "sync_play_group_members_sessions_sync_play_memberships" FOREIGN KEY ("session_id") REFERENCES "sessions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "sync_play_group_members_sync_play_groups_members" FOREIGN KEY ("group_id") REFERENCES "sync_play_groups" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "syncplaygroupmember_session_id" to table: "sync_play_group_members"
CREATE UNIQUE INDEX "syncplaygroupmember_session_id" ON "sync_play_group_members" ("session_id");
