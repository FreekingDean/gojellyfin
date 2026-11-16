SET LOCAL lock_timeout = '30s';
SET LOCAL client_connection_check_interval = '10s';
ALTER TABLE "user_policies" ALTER COLUMN "enable_live_tv_access" SET DEFAULT true;
CREATE TABLE IF NOT EXISTS "channels" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "number" integer NOT NULL, "name" character varying NOT NULL, "order" character varying NOT NULL DEFAULT 'Shuffle', "seed" bigint NOT NULL, "epoch" timestamptz NOT NULL, PRIMARY KEY ("id"));
CREATE UNIQUE INDEX IF NOT EXISTS "channels_number_key" ON "channels" ("number");
CREATE UNIQUE INDEX IF NOT EXISTS "channels_name_key" ON "channels" ("name");
CREATE TABLE IF NOT EXISTS "channel_entries" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "position" integer NOT NULL, "channel_id" uuid NOT NULL, "item_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "channel_entries_channels_entries" FOREIGN KEY ("channel_id") REFERENCES "channels" ("id") ON DELETE CASCADE, CONSTRAINT "channel_entries_items_channel_entries" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON DELETE CASCADE);
CREATE UNIQUE INDEX IF NOT EXISTS "channelentry_channel_id_item_id" ON "channel_entries" ("channel_id", "item_id");
CREATE INDEX IF NOT EXISTS "channelentry_item_id" ON "channel_entries" ("item_id");
