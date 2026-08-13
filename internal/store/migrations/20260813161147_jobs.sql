-- Create "job_schedules" table
CREATE TABLE IF NOT EXISTS "job_schedules" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "kind" character varying NOT NULL,
  "triggers" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "job_schedules_kind_key" to table: "job_schedules"
CREATE UNIQUE INDEX IF NOT EXISTS "job_schedules_kind_key" ON "job_schedules" ("kind");
-- Create "jobs" table
CREATE TABLE IF NOT EXISTS "jobs" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "kind" character varying NOT NULL,
  "state" character varying NOT NULL DEFAULT 'Queued',
  "payload" jsonb NULL,
  "dedupe_key" character varying NULL,
  "run_at" timestamptz NOT NULL,
  "attempt" bigint NOT NULL DEFAULT 0,
  "max_attempts" bigint NOT NULL DEFAULT 3,
  "progress" double precision NOT NULL DEFAULT 0,
  "cancel_requested" boolean NOT NULL DEFAULT false,
  "worker" character varying NULL,
  "lease_expires_at" timestamptz NULL,
  "started_at" timestamptz NULL,
  "finished_at" timestamptz NULL,
  "error_message" text NULL,
  PRIMARY KEY ("id")
);
-- Create index "job_kind_state" to table: "jobs"
CREATE INDEX IF NOT EXISTS "job_kind_state" ON "jobs" ("kind", "state");
-- Create index "job_state_run_at" to table: "jobs"
CREATE INDEX IF NOT EXISTS "job_state_run_at" ON "jobs" ("state", "run_at");
-- Create index "jobs_dedupe_key_key" to table: "jobs"
CREATE UNIQUE INDEX IF NOT EXISTS "jobs_dedupe_key_key" ON "jobs" ("dedupe_key");
