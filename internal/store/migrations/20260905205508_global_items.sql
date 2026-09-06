CREATE TABLE IF NOT EXISTS "library_items" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "item_id" uuid NOT NULL,
  "library_id" uuid NOT NULL,
  "source_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "library_items_items_libraries" FOREIGN KEY ("item_id") REFERENCES "items" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "library_items_libraries_library_items" FOREIGN KEY ("library_id") REFERENCES "libraries" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "library_items_sources_memberships" FOREIGN KEY ("source_id") REFERENCES "sources" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);

DROP TABLE IF EXISTS "merged";

DO $$ BEGIN
IF EXISTS (
  SELECT 1 FROM information_schema.columns
  WHERE table_schema = current_schema() AND table_name = 'items' AND column_name = 'library_id'
) THEN

-- Membership for what has a file, then for the seasons and series above them.
INSERT INTO "library_items" ("created_at", "updated_at", "library_id", "item_id", "source_id")
SELECT DISTINCT now(), now(), i.library_id, i.id, f.source_id
FROM "items" i JOIN "item_sources" f ON f.item_id = i.id
WHERE i.library_id IS NOT NULL;

INSERT INTO "library_items" ("created_at", "updated_at", "library_id", "item_id", "source_id")
SELECT DISTINCT now(), now(), m.library_id, parent.id, m.source_id
FROM "library_items" m
JOIN "items" child ON child.id = m.item_id
JOIN "items" parent ON parent.id = child.parent_id;

INSERT INTO "library_items" ("created_at", "updated_at", "library_id", "item_id", "source_id")
SELECT DISTINCT now(), now(), m.library_id, parent.id, m.source_id
FROM "library_items" m
JOIN "items" child ON child.id = m.item_id
JOIN "items" parent ON parent.id = child.parent_id;

-- One title is one row. Fold the copies other libraries held into the oldest.
CREATE TABLE "merged" AS
SELECT loser.id AS loser, keeper.id AS keeper
FROM "items" loser
JOIN (
  SELECT DISTINCT ON (key) key, id FROM "items" ORDER BY key, created_at, id
) keeper ON keeper.key = loser.key AND keeper.id <> loser.id;

INSERT INTO "user_item_data" (
  "id", "created_at", "updated_at", "user_id", "item_id",
  "played", "is_favorite", "play_count", "playback_position_ticks", "last_played_at"
)
SELECT gen_random_uuid(), now(), now(), d.user_id, m.keeper,
       bool_or(d.played), bool_or(d.is_favorite), max(d.play_count),
       max(d.playback_position_ticks), max(d.last_played_at)
FROM "user_item_data" d JOIN "merged" m ON m.loser = d.item_id
GROUP BY d.user_id, m.keeper
ON CONFLICT ("user_id", "item_id") DO UPDATE SET
  "played" = "user_item_data"."played" OR excluded."played",
  "is_favorite" = "user_item_data"."is_favorite" OR excluded."is_favorite",
  "play_count" = greatest("user_item_data"."play_count", excluded."play_count"),
  "playback_position_ticks" = greatest("user_item_data"."playback_position_ticks", excluded."playback_position_ticks"),
  "last_played_at" = greatest("user_item_data"."last_played_at", excluded."last_played_at");

UPDATE "items" c SET "parent_id" = m.keeper FROM "merged" m WHERE c.parent_id = m.loser;
UPDATE "item_sources" f SET "item_id" = m.keeper FROM "merged" m WHERE f.item_id = m.loser;
UPDATE "library_items" l SET "item_id" = m.keeper FROM "merged" m WHERE l.item_id = m.loser;
UPDATE "playlist_entries" e SET "item_id" = m.keeper FROM "merged" m WHERE e.item_id = m.loser;
UPDATE "activity_log_entries" a SET "item_activity_log_entries" = m.keeper FROM "merged" m WHERE a.item_activity_log_entries = m.loser;

UPDATE "images" g SET "item_id" = m.keeper
FROM "merged" m WHERE g.item_id = m.loser
  AND NOT EXISTS (
    SELECT 1 FROM "images" held
    WHERE held.item_id = m.keeper AND held.kind = g.kind AND held.index = g.index
  )
  AND g.id = (
    SELECT (array_agg(g2.id ORDER BY g2.id))[1] FROM "images" g2 JOIN "merged" m2 ON m2.loser = g2.item_id
    WHERE m2.keeper = m.keeper AND g2.kind = g.kind AND g2.index = g.index
  );

UPDATE "credits" c SET "item_credits" = m.keeper
FROM "merged" m WHERE c.item_credits = m.loser
  AND NOT EXISTS (
    SELECT 1 FROM "credits" held
    WHERE held.item_credits = m.keeper AND held.person_credits = c.person_credits
      AND held.kind = c.kind AND held.role = c.role
  )
  AND c.id = (
    SELECT (array_agg(c2.id ORDER BY c2.id))[1] FROM "credits" c2 JOIN "merged" m2 ON m2.loser = c2.item_credits
    WHERE m2.keeper = m.keeper AND c2.person_credits = c.person_credits
      AND c2.kind = c.kind AND c2.role = c.role
  );

INSERT INTO "item_genres" ("item_id", "genre_id")
SELECT DISTINCT m.keeper, g.genre_id
FROM "item_genres" g JOIN "merged" m ON m.loser = g.item_id
ON CONFLICT DO NOTHING;

INSERT INTO "item_studios" ("item_id", "studio_id")
SELECT DISTINCT m.keeper, g.studio_id
FROM "item_studios" g JOIN "merged" m ON m.loser = g.item_id
ON CONFLICT DO NOTHING;

DELETE FROM "items" WHERE "id" IN (SELECT loser FROM "merged");

DROP TABLE "merged";

DELETE FROM "library_items" a USING "library_items" b
WHERE a.library_id = b.library_id AND a.item_id = b.item_id AND a.source_id = b.source_id AND a.id > b.id;

END IF;
END $$;

ALTER TABLE "items" DROP COLUMN IF EXISTS "library_id";
CREATE UNIQUE INDEX IF NOT EXISTS "item_key" ON "items" ("key");
CREATE INDEX IF NOT EXISTS "libraryitem_item_id" ON "library_items" ("item_id");
CREATE UNIQUE INDEX IF NOT EXISTS "libraryitem_library_id_item_id_source_id" ON "library_items" ("library_id", "item_id", "source_id");
