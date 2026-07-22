-- Phase 2 continuation: unified evidence registry.
--
-- This migration is additive. It does not move or delete existing records,
-- KB assets, entities, or canonical rows. The registry is the evidence-first
-- catalog that links raw uploads, Knowledge Base entries, structured batches,
-- and later media/text processing results.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS forensic.evidence_items (
  evidence_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  case_id text,
  user_id text NOT NULL DEFAULT '',
  original_filename text NOT NULL,
  source_file text NOT NULL,
  content_type text,
  extension text,
  size_bytes bigint,
  sha256 text NOT NULL,
  modality text NOT NULL DEFAULT 'unknown',
  detected_type text NOT NULL DEFAULT 'unknown',
  classifier_confidence numeric(5,4) NOT NULL DEFAULT 0,
  processing_route text NOT NULL DEFAULT 'unknown',
  processing_status text NOT NULL DEFAULT 'registered',
  raw_storage_ref text,
  kb_entry_ref text,
  records_batch_id uuid,
  extracted_text_ref text,
  media_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  entities jsonb NOT NULL DEFAULT '[]'::jsonb,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  warnings jsonb NOT NULL DEFAULT '[]'::jsonb,
  errors jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT evidence_items_status_chk CHECK (
    processing_status IN ('registered', 'queued', 'processing', 'completed', 'failed', 'skipped', 'duplicate')
  ),
  CONSTRAINT evidence_items_confidence_chk CHECK (
    classifier_confidence >= 0 AND classifier_confidence <= 1
  )
);

CREATE INDEX IF NOT EXISTS evidence_items_collection_idx
  ON forensic.evidence_items (tenant_id, collection_id, created_at DESC);

CREATE INDEX IF NOT EXISTS evidence_items_hash_idx
  ON forensic.evidence_items (tenant_id, collection_id, sha256);

CREATE INDEX IF NOT EXISTS evidence_items_type_status_idx
  ON forensic.evidence_items (tenant_id, collection_id, modality, detected_type, processing_status);

CREATE INDEX IF NOT EXISTS evidence_items_source_file_idx
  ON forensic.evidence_items (tenant_id, collection_id, source_file);

CREATE INDEX IF NOT EXISTS evidence_items_metadata_gin_idx
  ON forensic.evidence_items USING gin (metadata);

CREATE INDEX IF NOT EXISTS evidence_items_entities_gin_idx
  ON forensic.evidence_items USING gin (entities);

ALTER TABLE forensic.records_ingest_jobs
  ADD COLUMN IF NOT EXISTS evidence_id uuid REFERENCES forensic.evidence_items(evidence_id);

ALTER TABLE forensic.kb_collection_assets
  ADD COLUMN IF NOT EXISTS evidence_id uuid REFERENCES forensic.evidence_items(evidence_id);

ALTER TABLE forensic.records
  ADD COLUMN IF NOT EXISTS evidence_id uuid REFERENCES forensic.evidence_items(evidence_id);

CREATE INDEX IF NOT EXISTS records_ingest_jobs_evidence_idx
  ON forensic.records_ingest_jobs (tenant_id, collection_id, evidence_id);

CREATE INDEX IF NOT EXISTS kb_collection_assets_evidence_idx
  ON forensic.kb_collection_assets (tenant_id, collection_id, evidence_id);

CREATE INDEX IF NOT EXISTS forensic_records_evidence_idx
  ON forensic.records (tenant_id, collection_id, evidence_id, "timestamp" DESC);

CREATE OR REPLACE FUNCTION forensic.touch_updated_at()
RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS evidence_items_touch_updated_at ON forensic.evidence_items;
CREATE TRIGGER evidence_items_touch_updated_at
  BEFORE UPDATE ON forensic.evidence_items
  FOR EACH ROW
  EXECUTE FUNCTION forensic.touch_updated_at();

ALTER TABLE forensic.evidence_items ENABLE ROW LEVEL SECURITY;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_policies
    WHERE schemaname = 'forensic'
      AND tablename = 'evidence_items'
      AND policyname = 'tenant_isolation_evidence_items'
  ) THEN
    CREATE POLICY tenant_isolation_evidence_items ON forensic.evidence_items
      USING (tenant_id = current_setting('app.tenant_id', true));
  END IF;
END $$;
