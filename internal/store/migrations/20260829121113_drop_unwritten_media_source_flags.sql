-- Modify "media_sources" table
ALTER TABLE "media_sources" DROP COLUMN "is_remote", DROP COLUMN "is_infinite_stream", DROP COLUMN "supports_transcoding", DROP COLUMN "supports_direct_stream", DROP COLUMN "supports_direct_play", DROP COLUMN "supports_probing", DROP COLUMN "requires_looping";
