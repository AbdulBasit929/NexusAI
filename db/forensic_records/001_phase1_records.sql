-- Phase 1 forensic records schema.
-- Target: PostgreSQL 16 + TimescaleDB. This schema is intentionally additive:
-- it does not modify LocalAI's existing Knowledge Base tables.

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE SCHEMA IF NOT EXISTS forensic;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'forensic_record_type') THEN
    CREATE TYPE forensic_record_type AS ENUM ('cdr', 'anpr', 'ipdr', 'subscriber', 'tower_location', 'transaction', 'access_log', 'generic');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'forensic_ingest_status') THEN
    CREATE TYPE forensic_ingest_status AS ENUM ('queued', 'running', 'completed', 'failed', 'dead_letter');
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS forensic.records_ingest_jobs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  user_id text NOT NULL DEFAULT '',
  collection_id text NOT NULL,
  file_id text NOT NULL,
  source_file text NOT NULL,
  spool_path text NOT NULL,
  sha256 text NOT NULL,
  record_type forensic_record_type NOT NULL DEFAULT 'generic',
  status forensic_ingest_status NOT NULL DEFAULT 'queued',
  attempt_count integer NOT NULL DEFAULT 0,
  max_attempts integer NOT NULL DEFAULT 5,
  total_rows bigint NOT NULL DEFAULT 0,
  accepted_rows bigint NOT NULL DEFAULT 0,
  duplicate_rows bigint NOT NULL DEFAULT 0,
  rejected_rows bigint NOT NULL DEFAULT 0,
  error_message text,
  queued_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz,
  completed_at timestamptz,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  CONSTRAINT records_ingest_jobs_attempts_chk CHECK (attempt_count >= 0 AND max_attempts > 0)
);

CREATE INDEX IF NOT EXISTS records_ingest_jobs_status_idx
  ON forensic.records_ingest_jobs (status, queued_at);
CREATE INDEX IF NOT EXISTS records_ingest_jobs_collection_idx
  ON forensic.records_ingest_jobs (tenant_id, collection_id, file_id);

CREATE TABLE IF NOT EXISTS forensic.kb_active_metadata (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  file_id text NOT NULL,
  batch_id uuid NOT NULL,
  record_type forensic_record_type NOT NULL,
  source_file text NOT NULL,
  sha256 text NOT NULL,
  min_timestamp timestamptz,
  max_timestamp timestamptz,
  total_rows bigint NOT NULL DEFAULT 0,
  inserted_rows bigint NOT NULL DEFAULT 0,
  duplicate_rows bigint NOT NULL DEFAULT 0,
  rejected_rows bigint NOT NULL DEFAULT 0,
  unique_targets_count bigint NOT NULL DEFAULT 0,
  unique_originators_count bigint NOT NULL DEFAULT 0,
  unique_locations_count bigint NOT NULL DEFAULT 0,
  normalized_schema jsonb NOT NULL DEFAULT '{}'::jsonb,
  quality_report jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, collection_id, file_id, batch_id)
);

CREATE INDEX IF NOT EXISTS kb_active_metadata_lookup_idx
  ON forensic.kb_active_metadata (tenant_id, collection_id, file_id, record_type);

CREATE TABLE IF NOT EXISTS forensic.cdr_records (
  id bigserial,
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  file_id text NOT NULL,
  batch_id uuid NOT NULL,
  row_number bigint NOT NULL,
  row_hash text NOT NULL,
  msisdn text,
  call_org_num text,
  call_dialed_num text,
  imsi text,
  imei text,
  call_start_ts timestamptz NOT NULL,
  call_end_ts timestamptz,
  duration_seconds integer,
  direction text,
  network_volume numeric,
  lac_id text,
  site_id text,
  cell_site_id text,
  latitude double precision,
  longitude double precision,
  call_type text,
  location text,
  raw_record jsonb NOT NULL,
  source_file text NOT NULL,
  ingested_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id, call_start_ts),
  UNIQUE (tenant_id, collection_id, file_id, row_hash, call_start_ts)
);

SELECT create_hypertable(
  'forensic.cdr_records',
  'call_start_ts',
  if_not_exists => TRUE,
  chunk_time_interval => INTERVAL '7 days'
);

CREATE INDEX IF NOT EXISTS cdr_records_batch_idx
  ON forensic.cdr_records (tenant_id, batch_id, row_number);
CREATE INDEX IF NOT EXISTS cdr_records_msisdn_time_idx
  ON forensic.cdr_records (tenant_id, msisdn, call_start_ts DESC);
CREATE INDEX IF NOT EXISTS cdr_records_target_time_idx
  ON forensic.cdr_records (tenant_id, call_dialed_num, call_start_ts DESC);
CREATE INDEX IF NOT EXISTS cdr_records_origin_time_idx
  ON forensic.cdr_records (tenant_id, call_org_num, call_start_ts DESC);
CREATE INDEX IF NOT EXISTS cdr_records_cell_time_idx
  ON forensic.cdr_records (tenant_id, cell_site_id, call_start_ts DESC);
CREATE INDEX IF NOT EXISTS cdr_records_type_time_idx
  ON forensic.cdr_records (tenant_id, call_type, call_start_ts DESC);
CREATE INDEX IF NOT EXISTS cdr_records_location_trgm_idx
  ON forensic.cdr_records USING gin (location gin_trgm_ops);

CREATE TABLE IF NOT EXISTS forensic.records_ingest_errors (
  id bigserial PRIMARY KEY,
  job_id uuid REFERENCES forensic.records_ingest_jobs(id) ON DELETE CASCADE,
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  file_id text NOT NULL,
  row_number bigint,
  error_code text NOT NULL,
  error_message text NOT NULL,
  raw_record jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS records_ingest_errors_job_idx
  ON forensic.records_ingest_errors (job_id, row_number);

CREATE TABLE IF NOT EXISTS forensic.records_audit_log (
  id bigserial PRIMARY KEY,
  tenant_id text NOT NULL DEFAULT 'default',
  user_id text NOT NULL DEFAULT '',
  action text NOT NULL,
  collection_id text,
  file_id text,
  batch_id uuid,
  details jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS records_audit_log_tenant_time_idx
  ON forensic.records_audit_log (tenant_id, created_at DESC);

ALTER TABLE forensic.records_ingest_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.kb_active_metadata ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.cdr_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.records_ingest_errors ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.records_audit_log ENABLE ROW LEVEL SECURITY;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = 'cdr_records' AND policyname = 'tenant_isolation_cdr') THEN
    CREATE POLICY tenant_isolation_cdr ON forensic.cdr_records
      USING (tenant_id = current_setting('app.tenant_id', true));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = 'kb_active_metadata' AND policyname = 'tenant_isolation_metadata') THEN
    CREATE POLICY tenant_isolation_metadata ON forensic.kb_active_metadata
      USING (tenant_id = current_setting('app.tenant_id', true));
  END IF;
END $$;

CREATE OR REPLACE VIEW forensic.cdr_frequent_contacts AS
SELECT
  tenant_id,
  collection_id,
  file_id,
  batch_id,
  call_dialed_num AS dialed_number,
  count(*) AS total_interactions,
  count(*) FILTER (WHERE direction = 'INCOMING') AS incoming_count,
  count(*) FILTER (WHERE direction = 'OUTGOING') AS outgoing_count,
  min(call_start_ts) AS first_contact,
  max(call_start_ts) AS last_contact
FROM forensic.cdr_records
WHERE call_dialed_num IS NOT NULL
GROUP BY tenant_id, collection_id, file_id, batch_id, call_dialed_num;
