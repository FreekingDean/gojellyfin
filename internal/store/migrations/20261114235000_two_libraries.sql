-- From here on there are exactly two libraries: Movies, fed by every Radarr, and Shows, fed by every Sonarr.
--
-- What happens to an install that has others:
--   * The oldest library whose collection type is movies is kept as Movies, and the oldest tvshows one as Shows,
--     ids and names and all, so a user's library access and the client's saved views keep pointing at them.
--   * With no such library, the oldest library a Radarr (or Sonarr) feeds becomes it, and with none of those either,
--     a new one is made under the id the server would give it.
--   * Every binding moves to the kept library its downloader's kind fills, and every membership with it, so no title
--     leaves the view of anyone who could see it. Two bindings of one downloader become one, which keeps a tag filter
--     only when both had the same one, since one library cannot show two subsets of one instance.
--   * A user limited to some libraries keeps access to the kept library that took over any library they could see.
--   * Every other library is then deleted. Its titles are not: every one it showed came from a downloader, and the
--     kept library that downloader now feeds shows it instead.
--   * Each kept library's folders become the folders its downloaders map, which is what saving a downloader keeps
--     them as from now on.
-- Running it again finds two libraries and changes nothing.
DO $$
DECLARE
  movies uuid;
  shows uuid;
BEGIN
  SELECT "id" INTO movies FROM "libraries" WHERE "collection_type" = 'movies' ORDER BY "created_at", "id" LIMIT 1;
  IF movies IS NULL THEN
    SELECT l."id" INTO movies FROM "libraries" l
    WHERE l."collection_type" <> 'tvshows'
      AND EXISTS (SELECT 1 FROM "library_sources" b JOIN "sources" s ON s."id" = b."source_id" WHERE b."library_id" = l."id" AND s."kind" = 'radarr')
    ORDER BY l."created_at", l."id" LIMIT 1;
    IF movies IS NOT NULL THEN
      UPDATE "libraries" SET "collection_type" = 'movies', "updated_at" = now() WHERE "id" = movies;
    ELSIF EXISTS (SELECT 1 FROM "library_sources" b JOIN "sources" s ON s."id" = b."source_id" WHERE s."kind" = 'radarr') THEN
      movies := '4d6f7669-6573-4000-8000-000000000001';
      INSERT INTO "libraries" ("id", "created_at", "updated_at", "name", "collection_type", "locations")
      VALUES (movies, now(), now(), movies::text, 'movies', '[]');
    END IF;
  END IF;

  SELECT "id" INTO shows FROM "libraries" WHERE "collection_type" = 'tvshows' ORDER BY "created_at", "id" LIMIT 1;
  IF shows IS NULL THEN
    SELECT l."id" INTO shows FROM "libraries" l
    WHERE l."id" IS DISTINCT FROM movies
      AND EXISTS (SELECT 1 FROM "library_sources" b JOIN "sources" s ON s."id" = b."source_id" WHERE b."library_id" = l."id" AND s."kind" = 'sonarr')
    ORDER BY l."created_at", l."id" LIMIT 1;
    IF shows IS NOT NULL THEN
      UPDATE "libraries" SET "collection_type" = 'tvshows', "updated_at" = now() WHERE "id" = shows;
    ELSIF EXISTS (SELECT 1 FROM "library_sources" b JOIN "sources" s ON s."id" = b."source_id" WHERE s."kind" = 'sonarr') THEN
      shows := '53686f77-7300-4000-8000-000000000002';
      INSERT INTO "libraries" ("id", "created_at", "updated_at", "name", "collection_type", "locations")
      VALUES (shows, now(), now(), shows::text, 'tvshows', '[]');
    END IF;
  END IF;

  CREATE TEMP TABLE "library_takeover" ON COMMIT DROP AS
  SELECT DISTINCT b."library_id" AS "old", CASE s."kind" WHEN 'radarr' THEN movies ELSE shows END AS "kept"
  FROM "library_sources" b JOIN "sources" s ON s."id" = b."source_id";

  UPDATE "user_policies" p SET "enabled_folders" = (
    SELECT COALESCE(jsonb_agg(DISTINCT kept), '[]'::jsonb) FROM (
      SELECT f::uuid AS kept FROM jsonb_array_elements_text(p."enabled_folders") f
      WHERE f::uuid IN (movies, shows)
      UNION
      SELECT t."kept" FROM "library_takeover" t
      WHERE t."kept" IS NOT NULL AND t."old"::text IN (SELECT jsonb_array_elements_text(p."enabled_folders"))
    ) granted
  )
  WHERE p."enabled_folders" IS NOT NULL AND jsonb_typeof(p."enabled_folders") = 'array';

  INSERT INTO "library_sources" ("created_at", "updated_at", "library_id", "source_id", "tag_filter", "last_synced_at", "last_error")
  SELECT now(), now(), kept, b."source_id",
    CASE WHEN count(DISTINCT COALESCE(b."tag_filter", '')) = 1 THEN max(COALESCE(b."tag_filter", '')) ELSE '' END,
    max(b."last_synced_at"), ''
  FROM "library_sources" b
  JOIN "sources" s ON s."id" = b."source_id"
  CROSS JOIN LATERAL (SELECT CASE s."kind" WHEN 'radarr' THEN movies ELSE shows END AS kept) k
  WHERE k.kept IS NOT NULL AND b."library_id" <> k.kept
  GROUP BY kept, b."source_id"
  ON CONFLICT ("library_id", "source_id") DO UPDATE SET
    "tag_filter" = CASE WHEN COALESCE("library_sources"."tag_filter", '') = EXCLUDED."tag_filter" THEN EXCLUDED."tag_filter" ELSE '' END,
    "updated_at" = now();

  DELETE FROM "library_sources" b USING "sources" s
  WHERE s."id" = b."source_id" AND b."library_id" IS DISTINCT FROM CASE s."kind" WHEN 'radarr' THEN movies ELSE shows END;

  INSERT INTO "library_items" ("created_at", "updated_at", "library_id", "item_id", "source_id")
  SELECT now(), now(), k.kept, m."item_id", m."source_id"
  FROM "library_items" m
  JOIN "sources" s ON s."id" = m."source_id"
  CROSS JOIN LATERAL (SELECT CASE s."kind" WHEN 'radarr' THEN movies ELSE shows END AS kept) k
  WHERE k.kept IS NOT NULL AND m."library_id" <> k.kept
  ON CONFLICT ("library_id", "item_id", "source_id") DO NOTHING;

  DELETE FROM "library_items" m USING "sources" s
  WHERE s."id" = m."source_id" AND m."library_id" IS DISTINCT FROM CASE s."kind" WHEN 'radarr' THEN movies ELSE shows END;

  DELETE FROM "libraries" WHERE "id" NOT IN (SELECT x FROM (VALUES (movies), (shows)) v(x) WHERE x IS NOT NULL);

  UPDATE "libraries" l SET "name" = CASE l."collection_type" WHEN 'movies' THEN 'Movies' ELSE 'Shows' END
  WHERE l."name" = l."id"::text
    AND NOT EXISTS (SELECT 1 FROM "libraries" o WHERE o."name" = CASE l."collection_type" WHEN 'movies' THEN 'Movies' ELSE 'Shows' END);

  UPDATE "libraries" l SET "locations" = COALESCE((
    SELECT jsonb_agg(DISTINCT p."local_path" ORDER BY p."local_path")
    FROM "source_paths" p JOIN "library_sources" b ON b."source_id" = p."source_id"
    WHERE b."library_id" = l."id"
  ), '[]'::jsonb), "updated_at" = now()
  WHERE EXISTS (SELECT 1 FROM "library_sources" b WHERE b."library_id" = l."id");
END $$;
