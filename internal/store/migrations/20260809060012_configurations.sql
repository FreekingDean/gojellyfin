-- Create "configurations" table
CREATE TABLE "configurations" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "key" character varying NOT NULL,
  "value" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "configurations_key_key" to table: "configurations"
CREATE UNIQUE INDEX "configurations_key_key" ON "configurations" ("key");
