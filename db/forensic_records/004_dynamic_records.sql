-- Phase 3: canonical schema-agnostic records table.
-- This table is additive. Existing cdr_records, generic_records, entities,
-- metadata, and KB asset tables remain the compatibility/query surface.

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS forensic.records (
  record_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id varchar NOT NULL DEFAULT 'default',
  collection_id varchar NOT NULL,
  file_id varchar NOT NULL,
  batch_id uuid NOT NULL,
  record_type varchar NOT NULL DEFAULT 'generic',
  "timestamp" timestamptz NOT NULL DEFAULT now(),
  primary_target varchar,
  secondary_target varchar,
  source_file varchar NOT NULL,
  row_number bigint NOT NULL,
  row_hash varchar NOT NULL,
  raw_payload jsonb NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  ingested_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (record_id, "timestamp"),
  UNIQUE (tenant_id, collection_id, file_id, batch_id, row_hash, "timestamp")
);

SELECT create_hypertable(
  'forensic.records',
  'timestamp',
  if_not_exists => TRUE,
  chunk_time_interval => INTERVAL '7 days'
);

CREATE INDEX IF NOT EXISTS forensic_records_lookup_idx
  ON forensic.records (tenant_id, collection_id, record_type, "timestamp" DESC, primary_target);

CREATE INDEX IF NOT EXISTS forensic_records_secondary_target_idx
  ON forensic.records (tenant_id, collection_id, secondary_target, "timestamp" DESC);

CREATE INDEX IF NOT EXISTS forensic_records_payload_gin_idx
  ON forensic.records USING gin (raw_payload);

ALTER TABLE forensic.records ENABLE ROW LEVEL SECURITY;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_policies
    WHERE schemaname = 'forensic'
      AND tablename = 'records'
      AND policyname = 'tenant_isolation_records'
  ) THEN
    CREATE POLICY tenant_isolation_records ON forensic.records
      USING (tenant_id = current_setting('app.tenant_id', true));
  END IF;
END $$;
