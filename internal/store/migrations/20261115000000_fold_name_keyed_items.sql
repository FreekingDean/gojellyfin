-- Titles scanned before the downloaders fed the library were keyed by name, `movie:the-matrix:1999`. The first
-- refresh from Sonarr and Radarr keyed each again by its TMDB id, and saving its file by path moved the file onto
-- the new row, leaving the old one in every listing with nothing to play and every watch state still on it.
--
-- What happens here:
--   * An old row that has no file left is paired with the TMDB keyed row of the same kind whose name matches once
--     case and punctuation are dropped: a movie by year as well, a series by year where both have one, a season by
--     number under its paired series, and an episode by number under its paired season. A row that pairs with more
--     than one candidate, or none, is not paired.
--   * Each pair is folded the way the global items migration folded copies: watch state is merged (played and
--     favourite OR'd, the further count and position kept), and playlist entries, reports, activity, SyncPlay queue
--     entries, children and whatever artwork the new row lacks move over. The old row is then deleted.
--   * An old row left with no file and no pair is marked deleted, which is what the sweep would have done with it,
--     and so is a series or season left with nothing living beneath it.
-- Running it again finds no old rows without a file and changes nothing.
DO $$
BEGIN
  CREATE TEMP TABLE "name_keyed" ON COMMIT DROP AS
  SELECT i."id", i."kind", i."parent_id", i."index_number", i."production_year",
         regexp_replace(lower(i."name"), '[^[:alnum:]]+', '', 'g') AS "plain"
  FROM "items" i
  WHERE i."deleted_at" IS NULL
    AND i."kind" IN ('Movie', 'Series', 'Season', 'Episode')
    AND i."key" !~ '^(movie|series|season|episode):tmdb:'
    AND NOT EXISTS (
      SELECT 1 FROM "items" e JOIN "item_sources" f ON f."item_id" = e."id"
      WHERE e."id" = i."id" OR e."parent_id" = i."id"
         OR e."parent_id" IN (SELECT s."id" FROM "items" s WHERE s."parent_id" = i."id")
    );

  CREATE TEMP TABLE "tmdb_keyed" ON COMMIT DROP AS
  SELECT i."id", i."kind", i."parent_id", i."index_number", i."production_year",
         regexp_replace(lower(i."name"), '[^[:alnum:]]+', '', 'g') AS "plain"
  FROM "items" i
  WHERE i."deleted_at" IS NULL AND i."key" ~ '^(movie|series|season|episode):tmdb:';

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
  FROM "folded" f WHERE c."parent_id" = f."loser" AND c."id" NOT IN (SELECT "loser" FROM "folded");

  UPDATE "images" g SET "item_id" = f."keeper"
  FROM "folded" f WHERE g."item_id" = f."loser"
    AND NOT EXISTS (
      SELECT 1 FROM "images" held
      WHERE held."item_id" = f."keeper" AND held."kind" = g."kind" AND held."index" = g."index"
    )
    AND g."id" = (
      SELECT (array_agg(g2."id" ORDER BY g2."id"))[1] FROM "images" g2 JOIN "folded" f2 ON f2."loser" = g2."item_id"
      WHERE f2."keeper" = f."keeper" AND g2."kind" = g."kind" AND g2."index" = g."index"
    );

  DELETE FROM "items" WHERE "id" IN (SELECT "loser" FROM "folded");

  UPDATE "items" SET "deleted_at" = now(), "updated_at" = now()
  WHERE "id" IN (SELECT "id" FROM "name_keyed") AND "kind" IN ('Movie', 'Episode');

  LOOP
    UPDATE "items" i SET "deleted_at" = now(), "updated_at" = now()
    WHERE i."deleted_at" IS NULL
      AND i."kind" IN ('Series', 'Season')
      AND EXISTS (SELECT 1 FROM "items" c WHERE c."parent_id" = i."id")
      AND NOT EXISTS (SELECT 1 FROM "items" c WHERE c."parent_id" = i."id" AND c."deleted_at" IS NULL);
    EXIT WHEN NOT FOUND;
  END LOOP;
END $$;
