-- Titles scanned before the downloaders fed the library were keyed by name, `movie:the-matrix:1999`. The first
-- refresh from Sonarr and Radarr keyed each again by its TMDB id, and saving its file by path moved the file onto
-- the new row, leaving the old one in every listing with nothing to play and every watch state still on it.
--
-- What happens here:
--   * The foreign keys into items that a delete cascades or nulls through gain the indexes they lacked, so removing
--     a row is an index probe per table rather than a scan of each.
--   * An old row with no file on it or anywhere beneath it is paired with the TMDB keyed row of the same kind whose
--     name matches once case and punctuation are dropped: a movie by year as well, a series by year where both have
--     one, a season by number under its paired series, and an episode by number under its paired season. A row
--     that pairs with more than one candidate, or none, is not paired.
--   * Each pair is folded the way the global items migration folded copies: watch state is merged (played and
--     favourite OR'd, the further count and position kept), and playlist entries, reports, activity, SyncPlay queue
--     entries, children and whatever artwork the new row lacks move over. The old row is then deleted.
--   * Every old row left over has no file beneath it, so it is marked deleted, which is what the sweep would have
--     done with it.
-- Every step is a set join, and a lock held longer than the timeout fails the run rather than stalling a live server
-- behind it, and a run whose client has gone away, its Job deleted, notices within ten seconds and rolls back rather
-- holding its locks against the next one. Running it again finds no old rows without a file and changes nothing.
SET LOCAL lock_timeout = '30s';
SET LOCAL client_connection_check_interval = '10s';

CREATE INDEX IF NOT EXISTS "item_parent_id" ON "items" ("parent_id");
CREATE INDEX IF NOT EXISTS "useritemdata_item_id" ON "user_item_data" ("item_id");
CREATE INDEX IF NOT EXISTS "playlistentry_item_id" ON "playlist_entries" ("item_id");
CREATE INDEX IF NOT EXISTS "credit_item_credits" ON "credits" ("item_credits");
CREATE INDEX IF NOT EXISTS "activitylogentry_item_activity_log_entries" ON "activity_log_entries" ("item_activity_log_entries");
CREATE INDEX IF NOT EXISTS "session_now_playing_item_id" ON "sessions" ("now_playing_item_id");

CREATE TEMP TABLE "held" ON COMMIT DROP AS
SELECT DISTINCT h."id"
FROM "item_sources" f
JOIN "items" e ON e."id" = f."item_id"
LEFT JOIN "items" p ON p."id" = e."parent_id"
CROSS JOIN LATERAL (VALUES (e."id"), (e."parent_id"), (p."parent_id")) AS h ("id")
WHERE h."id" IS NOT NULL;

CREATE TEMP TABLE "name_keyed" ON COMMIT DROP AS
SELECT i."id", i."kind", i."parent_id", i."index_number", i."production_year",
       regexp_replace(lower(i."name"), '[^[:alnum:]]+', '', 'g') AS "plain"
FROM "items" i
WHERE i."deleted_at" IS NULL
  AND i."kind" IN ('Movie', 'Series', 'Season', 'Episode')
  AND i."key" !~ '^(movie|series|season|episode):tmdb:'
  AND NOT EXISTS (SELECT 1 FROM "held" h WHERE h."id" = i."id");

CREATE TEMP TABLE "tmdb_keyed" ON COMMIT DROP AS
SELECT i."id", i."kind", i."parent_id", i."index_number", i."production_year",
       regexp_replace(lower(i."name"), '[^[:alnum:]]+', '', 'g') AS "plain"
FROM "items" i
WHERE i."deleted_at" IS NULL AND i."key" ~ '^(movie|series|season|episode):tmdb:';

ANALYZE "held";
ANALYZE "name_keyed";
ANALYZE "tmdb_keyed";

CREATE TEMP TABLE "folded" ("loser" uuid PRIMARY KEY, "keeper" uuid NOT NULL) ON COMMIT DROP;

INSERT INTO "folded"
SELECT o."id", min(n."id"::text)::uuid
FROM "name_keyed" o JOIN "tmdb_keyed" n
  ON n."kind" = o."kind" AND n."plain" = o."plain" AND n."plain" <> ''
 AND (o."kind" = 'Movie' AND n."production_year" IS NOT DISTINCT FROM o."production_year"
   OR o."kind" = 'Series' AND (n."production_year" IS NULL OR o."production_year" IS NULL OR n."production_year" = o."production_year"))
WHERE o."kind" IN ('Movie', 'Series')
GROUP BY o."id"
HAVING count(*) = 1;

INSERT INTO "folded"
SELECT o."id", min(n."id"::text)::uuid
FROM "name_keyed" o
JOIN "folded" p ON p."loser" = o."parent_id"
JOIN "tmdb_keyed" n ON n."kind" = 'Season' AND n."parent_id" = p."keeper" AND n."index_number" = o."index_number"
WHERE o."kind" = 'Season'
GROUP BY o."id"
HAVING count(*) = 1;

INSERT INTO "folded"
SELECT o."id", min(n."id"::text)::uuid
FROM "name_keyed" o
JOIN "folded" p ON p."loser" = o."parent_id"
JOIN "tmdb_keyed" n ON n."kind" = 'Episode' AND n."parent_id" = p."keeper" AND n."index_number" = o."index_number"
WHERE o."kind" = 'Episode'
GROUP BY o."id"
HAVING count(*) = 1;

ANALYZE "folded";

INSERT INTO "user_item_data" (
  "id", "created_at", "updated_at", "user_id", "item_id",
  "played", "is_favorite", "play_count", "playback_position_ticks", "last_played_at"
)
SELECT gen_random_uuid(), now(), now(), d."user_id", f."keeper",
       bool_or(d."played"), bool_or(d."is_favorite"), max(d."play_count"),
       max(d."playback_position_ticks"), max(d."last_played_at")
FROM "user_item_data" d JOIN "folded" f ON f."loser" = d."item_id"
GROUP BY d."user_id", f."keeper"
ON CONFLICT ("user_id", "item_id") DO UPDATE SET
  "played" = "user_item_data"."played" OR excluded."played",
  "is_favorite" = "user_item_data"."is_favorite" OR excluded."is_favorite",
  "play_count" = greatest("user_item_data"."play_count", excluded."play_count"),
  "playback_position_ticks" = greatest("user_item_data"."playback_position_ticks", excluded."playback_position_ticks"),
  "last_played_at" = greatest("user_item_data"."last_played_at", excluded."last_played_at"),
  "updated_at" = now();

UPDATE "playlist_entries" e SET "item_id" = f."keeper" FROM "folded" f WHERE e."item_id" = f."loser";
UPDATE "reports" r SET "item_id" = f."keeper" FROM "folded" f WHERE r."item_id" = f."loser";
UPDATE "sync_play_queue_entries" q SET "item_id" = f."keeper" FROM "folded" f WHERE q."item_id" = f."loser";
UPDATE "activity_log_entries" a SET "item_activity_log_entries" = f."keeper"
FROM "folded" f WHERE a."item_activity_log_entries" = f."loser";
UPDATE "items" c SET "parent_id" = f."keeper", "updated_at" = now()
FROM "folded" f
WHERE c."parent_id" = f."loser"
  AND NOT EXISTS (SELECT 1 FROM "folded" moving WHERE moving."loser" = c."id");

UPDATE "images" g SET "item_id" = m."keeper"
FROM (
  SELECT DISTINCT ON (f."keeper", g."kind", g."index") g."id", f."keeper"
  FROM "images" g JOIN "folded" f ON f."loser" = g."item_id"
  WHERE NOT EXISTS (
    SELECT 1 FROM "images" taken
    WHERE taken."item_id" = f."keeper" AND taken."kind" = g."kind" AND taken."index" = g."index"
  )
  ORDER BY f."keeper", g."kind", g."index", g."id"
) m
WHERE g."id" = m."id";

DELETE FROM "items" i USING "folded" f WHERE i."id" = f."loser";

UPDATE "items" i SET "deleted_at" = now(), "updated_at" = now()
FROM "name_keyed" n
WHERE i."id" = n."id" AND i."deleted_at" IS NULL;
