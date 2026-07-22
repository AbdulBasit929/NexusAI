-- Phase 2: make Knowledge Base collections the product surface while keeping
-- structured records queryable through deterministic analytical tables.

CREATE TABLE IF NOT EXISTS forensic.kb_collection_assets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  user_id text NOT NULL DEFAULT '',
  collection_id text NOT NULL,
  file_id text NOT NULL,
  batch_id uuid,
  source_file text NOT NULL,
  source_entry text,
  sha256 text NOT NULL,
  detected_record_type forensic_record_type NOT NULL DEFAULT 'generic',
  requested_record_type forensic_record_type NOT NULL DEFAULT 'generic',
  storage_mode text NOT NULL DEFAULT 'hybrid',
  rag_status text NOT NULL DEFAULT 'pending',
  structured_status text NOT NULL DEFAULT 'pending',
  content_type text,
  size_bytes bigint,
  headers jsonb NOT NULL DEFAULT '[]'::jsonb,
  routing_decision jsonb NOT NULL DEFAULT '{}'::jsonb,
  quality_report jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT kb_collection_assets_storage_mode_chk
    CHECK (storage_mode IN ('rag_only', 'records_only', 'hybrid')),
  CONSTRAINT kb_collection_assets_rag_status_chk
    CHECK (rag_status IN ('pending', 'mirrored', 'skipped', 'failed')),
  CONSTRAINT kb_collection_assets_structured_status_chk
    CHECK (structured_status IN ('pending', 'completed', 'failed', 'skipped')),
  UNIQUE (tenant_id, collection_id, file_id, sha256)
);

CREATE INDEX IF NOT EXISTS kb_collection_assets_lookup_idx
  ON forensic.kb_collection_assets (tenant_id, collection_id, detected_record_type, created_at DESC);

CREATE TABLE IF NOT EXISTS forensic.generic_records (
  id bigserial PRIMARY KEY,
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  file_id text NOT NULL,
  batch_id uuid NOT NULL,
  record_type forensic_record_type NOT NULL DEFAULT 'generic',
  row_number bigint NOT NULL,
  row_hash text NOT NULL,
  observed_at timestamptz,
  primary_entity text,
  secondary_entity text,
  location text,
  latitude double precision,
  longitude double precision,
  raw_record jsonb NOT NULL,
  source_file text NOT NULL,
  ingested_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, collection_id, file_id, batch_id, row_hash)
);

CREATE INDEX IF NOT EXISTS generic_records_batch_idx
  ON forensic.generic_records (tenant_id, batch_id, row_number);
CREATE INDEX IF NOT EXISTS generic_records_type_time_idx
  ON forensic.generic_records (tenant_id, record_type, observed_at DESC);
CREATE INDEX IF NOT EXISTS generic_records_primary_entity_idx
  ON forensic.generic_records (tenant_id, primary_entity, observed_at DESC);
CREATE INDEX IF NOT EXISTS generic_records_location_trgm_idx
  ON forensic.generic_records USING gin (location gin_trgm_ops);

CREATE TABLE IF NOT EXISTS forensic.record_entities (
  id bigserial PRIMARY KEY,
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  file_id text NOT NULL,
  batch_id uuid NOT NULL,
  record_type forensic_record_type NOT NULL,
  row_number bigint NOT NULL,
  row_hash text NOT NULL,
  observed_at timestamptz,
  entity_type text NOT NULL,
  entity_value text NOT NULL,
  source_field text NOT NULL,
  source_file text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (
    tenant_id, collection_id, file_id, batch_id,
    record_type, row_hash, entity_type, entity_value, source_field
  )
);

CREATE INDEX IF NOT EXISTS record_entities_entity_idx
  ON forensic.record_entities (tenant_id, entity_type, entity_value, observed_at DESC);
CREATE INDEX IF NOT EXISTS record_entities_batch_idx
  ON forensic.record_entities (tenant_id, batch_id, row_number);

ALTER TABLE forensic.kb_collection_assets ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.generic_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.record_entities ENABLE ROW LEVEL SECURITY;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = 'kb_collection_assets' AND policyname = 'tenant_isolation_kb_assets') THEN
    CREATE POLICY tenant_isolation_kb_assets ON forensic.kb_collection_assets
      USING (tenant_id = current_setting('app.tenant_id', true));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = 'generic_records' AND policyname = 'tenant_isolation_generic_records') THEN
    CREATE POLICY tenant_isolation_generic_records ON forensic.generic_records
      USING (tenant_id = current_setting('app.tenant_id', true));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = 'record_entities' AND policyname = 'tenant_isolation_record_entities') THEN
    CREATE POLICY tenant_isolation_record_entities ON forensic.record_entities
      USING (tenant_id = current_setting('app.tenant_id', true));
  END IF;
END $$;

CREATE OR REPLACE VIEW forensic.entity_activity_summary AS
SELECT
  tenant_id,
  collection_id,
  entity_type,
  entity_value,
  count(*) AS observation_count,
  count(DISTINCT record_type) AS record_type_count,
  min(observed_at) AS first_seen,
  max(observed_at) AS last_seen,
  array_agg(DISTINCT record_type::text ORDER BY record_type::text) AS record_types
FROM forensic.record_entities
GROUP BY tenant_id, collection_id, entity_type, entity_value;
