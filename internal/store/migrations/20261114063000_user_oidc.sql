-- Modify "users" table
ALTER TABLE "users" ADD COLUMN IF NOT EXISTS "oidc_issuer" character varying NULL, ADD COLUMN IF NOT EXISTS "oidc_subject" character varying NULL;
-- Create index "user_oidc_issuer_oidc_subject" to table: "users"
CREATE UNIQUE INDEX IF NOT EXISTS "user_oidc_issuer_oidc_subject" ON "users" ("oidc_issuer", "oidc_subject");
