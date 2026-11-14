-- Modify "items" table
ALTER TABLE "items" DROP CONSTRAINT IF EXISTS "item_kind_known";
ALTER TABLE "items" ADD CONSTRAINT "item_kind_known" CHECK ((kind)::text = ANY ((ARRAY['Movie'::character varying, 'Series'::character varying, 'Season'::character varying, 'Episode'::character varying, 'Playlist'::character varying, 'MusicArtist'::character varying, 'MusicAlbum'::character varying, 'Audio'::character varying])::text[]));
