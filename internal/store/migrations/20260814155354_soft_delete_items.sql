-- Modify "items" table
ALTER TABLE "items" ADD COLUMN "deleted_at" timestamptz NULL;
-- Create index "item_deleted_at" to table: "items"
CREATE INDEX "item_deleted_at" ON "items" ("deleted_at");
