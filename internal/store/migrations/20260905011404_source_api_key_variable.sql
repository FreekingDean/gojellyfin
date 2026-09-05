-- Modify "sources" table
ALTER TABLE "sources" DROP COLUMN "api_key";
ALTER TABLE "sources" ADD COLUMN "api_key_variable" character varying NOT NULL DEFAULT '';
ALTER TABLE "sources" ALTER COLUMN "api_key_variable" DROP DEFAULT;
