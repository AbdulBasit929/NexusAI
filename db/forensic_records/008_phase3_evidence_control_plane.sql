-- Phase 3: normalized evidence versions, immutable storage references,
-- processing runs/events, derived artifacts, and chain-of-custody events.
--
-- This migration is additive. Existing evidence, jobs, KB assets, canonical
-- records, and audit rows remain authoritative and are linked into the new
-- control plane. Apply only after a scoped database backup and with
-- ON_ERROR_STOP enabled.

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE UNIQUE INDEX IF NOT EXISTS evidence_items_tenant_identity_idx
  ON forensic.evidence_items (tenant_id, collection_id, evidence_id);

CREATE TABLE IF NOT EXISTS forensic.evidence_storage_objects (
  storage_object_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL,
  collection_id text NOT NULL,
  content_sha256 text NOT NULL,
  size_bytes bigint,
  storage_backend text NOT NULL,
  storage_uri text NOT NULL,
  immutability_state text NOT NULL DEFAULT 'unverified',
  write_once boolean NOT NULL DEFAULT false,
  legal_hold boolean NOT NULL DEFAULT false,
  retention_until timestamptz,
  verified_at timestamptz,
  verification_method text,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT evidence_storage_objects_hash_chk CHECK (content_sha256 ~ '^[0-9a-f]{64}$'),
  CONSTRAINT evidence_storage_objects_size_chk CHECK (size_bytes IS NULL OR size_bytes >= 0),
  CONSTRAINT evidence_storage_objects_state_chk CHECK (
    immutability_state IN ('pending', 'unverified', 'verified', 'failed')
  ),
  CONSTRAINT evidence_storage_objects_metadata_chk CHECK (jsonb_typeof(metadata) = 'object'),
  UNIQUE (tenant_id, collection_id, content_sha256, storage_uri),
  UNIQUE (tenant_id, collection_id, storage_object_id)
);

CREATE INDEX IF NOT EXISTS evidence_storage_objects_hash_idx
  ON forensic.evidence_storage_objects (tenant_id, collection_id, content_sha256);

CREATE INDEX IF NOT EXISTS evidence_storage_objects_retention_idx
  ON forensic.evidence_storage_objects (tenant_id, legal_hold, retention_until);

CREATE TABLE IF NOT EXISTS forensic.evidence_versions (
  version_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL,
  collection_id text NOT NULL,
  evidence_id uuid NOT NULL,
  version_number bigint NOT NULL,
  previous_version_id uuid,
  storage_object_id uuid NOT NULL,
  content_sha256 text NOT NULL,
  size_bytes bigint,
  media_type text,
  source_name text NOT NULL,
  version_reason text NOT NULL DEFAULT 'initial_registration',
  created_by text NOT NULL DEFAULT '',
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT evidence_versions_number_chk CHECK (version_number > 0),
  CONSTRAINT evidence_versions_hash_chk CHECK (content_sha256 ~ '^[0-9a-f]{64}$'),
  CONSTRAINT evidence_versions_size_chk CHECK (size_bytes IS NULL OR size_bytes >= 0),
  CONSTRAINT evidence_versions_metadata_chk CHECK (jsonb_typeof(metadata) = 'object'),
  CONSTRAINT evidence_versions_evidence_fk FOREIGN KEY (tenant_id, collection_id, evidence_id)
    REFERENCES forensic.evidence_items (tenant_id, collection_id, evidence_id),
  CONSTRAINT evidence_versions_storage_fk FOREIGN KEY (tenant_id, collection_id, storage_object_id)
    REFERENCES forensic.evidence_storage_objects (tenant_id, collection_id, storage_object_id),
  UNIQUE (tenant_id, collection_id, evidence_id, version_number),
  UNIQUE (tenant_id, collection_id, evidence_id, version_id)
);

ALTER TABLE forensic.evidence_versions
  DROP CONSTRAINT IF EXISTS evidence_versions_previous_fk;
ALTER TABLE forensic.evidence_versions
  ADD CONSTRAINT evidence_versions_previous_fk
  FOREIGN KEY (tenant_id, collection_id, evidence_id, previous_version_id)
  REFERENCES forensic.evidence_versions (tenant_id, collection_id, evidence_id, version_id)
  DEFERRABLE INITIALLY DEFERRED;

CREATE INDEX IF NOT EXISTS evidence_versions_hash_idx
  ON forensic.evidence_versions (tenant_id, collection_id, content_sha256);

ALTER TABLE forensic.evidence_items
  ADD COLUMN IF NOT EXISTS current_version_id uuid;

CREATE TABLE IF NOT EXISTS forensic.evidence_source_links (
  source_link_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL,
  collection_id text NOT NULL,
  evidence_id uuid NOT NULL,
  version_id uuid NOT NULL,
  source_kind text NOT NULL,
  source_uri text NOT NULL,
  source_name text NOT NULL,
  external_source_id text,
  relationship text NOT NULL DEFAULT 'source',
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  observed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT evidence_source_links_kind_chk CHECK (
    source_kind IN ('upload', 'knowledge_base', 'external', 'legacy', 'derived')
  ),
  CONSTRAINT evidence_source_links_relationship_chk CHECK (
    relationship IN ('source', 'mirror', 'import', 'export', 'derivation')
  ),
  CONSTRAINT evidence_source_links_metadata_chk CHECK (jsonb_typeof(metadata) = 'object'),
  CONSTRAINT evidence_source_links_version_fk FOREIGN KEY (tenant_id, collection_id, evidence_id, version_id)
    REFERENCES forensic.evidence_versions (tenant_id, collection_id, evidence_id, version_id),
  UNIQUE (tenant_id, collection_id, source_link_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS evidence_source_links_identity_idx
  ON forensic.evidence_source_links (
    tenant_id, collection_id, version_id, source_kind, source_uri,
    coalesce(external_source_id, '')
  );

CREATE INDEX IF NOT EXISTS evidence_source_links_evidence_idx
  ON forensic.evidence_source_links (tenant_id, collection_id, evidence_id, version_id);

CREATE TABLE IF NOT EXISTS forensic.processing_runs (
  run_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL,
  collection_id text NOT NULL,
  evidence_id uuid NOT NULL,
  version_id uuid NOT NULL,
  parent_run_id uuid,
  run_kind text NOT NULL,
  pipeline_id text NOT NULL,
  pipeline_revision text,
  adapter_id text,
  adapter_revision text,
  model_id text,
  model_revision text,
  parameters jsonb NOT NULL DEFAULT '{}'::jsonb,
  parameters_sha256 text,
  idempotency_key text NOT NULL,
  status text NOT NULL DEFAULT 'requested',
  attempt_count integer NOT NULL DEFAULT 0,
  requested_by text NOT NULL DEFAULT '',
  requested_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz,
  completed_at timestamptz,
  input_sha256 text NOT NULL,
  output_manifest jsonb NOT NULL DEFAULT '{}'::jsonb,
  error jsonb NOT NULL DEFAULT '{}'::jsonb,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  CONSTRAINT processing_runs_kind_chk CHECK (
    run_kind IN ('ingest', 'extract', 'index', 'verify', 'query', 'report', 'reprocess')
  ),
  CONSTRAINT processing_runs_status_chk CHECK (
    status IN ('requested', 'queued', 'running', 'succeeded', 'failed', 'cancelled', 'dead_letter')
  ),
  CONSTRAINT processing_runs_attempt_chk CHECK (attempt_count >= 0),
  CONSTRAINT processing_runs_input_hash_chk CHECK (input_sha256 ~ '^[0-9a-f]{64}$'),
  CONSTRAINT processing_runs_parameters_hash_chk CHECK (
    parameters_sha256 IS NULL OR parameters_sha256 ~ '^[0-9a-f]{64}$'
  ),
  CONSTRAINT processing_runs_json_chk CHECK (
    jsonb_typeof(parameters) = 'object'
    AND jsonb_typeof(output_manifest) = 'object'
    AND jsonb_typeof(error) = 'object'
    AND jsonb_typeof(metadata) = 'object'
  ),
  CONSTRAINT processing_runs_time_chk CHECK (
    (started_at IS NULL OR started_at >= requested_at)
    AND (completed_at IS NULL OR started_at IS NULL OR completed_at >= started_at)
  ),
  CONSTRAINT processing_runs_version_fk FOREIGN KEY (tenant_id, collection_id, evidence_id, version_id)
    REFERENCES forensic.evidence_versions (tenant_id, collection_id, evidence_id, version_id),
  UNIQUE (tenant_id, collection_id, run_id),
  UNIQUE (tenant_id, collection_id, idempotency_key)
);

ALTER TABLE forensic.processing_runs
  DROP CONSTRAINT IF EXISTS processing_runs_parent_fk;
ALTER TABLE forensic.processing_runs
  ADD CONSTRAINT processing_runs_parent_fk
  FOREIGN KEY (tenant_id, collection_id, parent_run_id)
  REFERENCES forensic.processing_runs (tenant_id, collection_id, run_id)
  DEFERRABLE INITIALLY DEFERRED;

CREATE INDEX IF NOT EXISTS processing_runs_evidence_idx
  ON forensic.processing_runs (tenant_id, collection_id, evidence_id, version_id, requested_at DESC);

CREATE INDEX IF NOT EXISTS processing_runs_status_idx
  ON forensic.processing_runs (tenant_id, status, requested_at);

CREATE TABLE IF NOT EXISTS forensic.derived_artifacts (
  artifact_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL,
  collection_id text NOT NULL,
  evidence_id uuid NOT NULL,
  version_id uuid NOT NULL,
  run_id uuid NOT NULL,
  parent_artifact_id uuid,
  storage_object_id uuid,
  artifact_type text NOT NULL,
  media_type text,
  content_sha256 text,
  processing_status text NOT NULL DEFAULT 'registered',
  confidence numeric(7,6),
  citation_locator jsonb NOT NULL DEFAULT '{}'::jsonb,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  warnings jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT derived_artifacts_hash_chk CHECK (
    content_sha256 IS NULL OR content_sha256 ~ '^[0-9a-f]{64}$'
  ),
  CONSTRAINT derived_artifacts_status_chk CHECK (
    processing_status IN ('registered', 'processing', 'completed', 'failed', 'superseded')
  ),
  CONSTRAINT derived_artifacts_confidence_chk CHECK (
    confidence IS NULL OR (confidence >= 0 AND confidence <= 1)
  ),
  CONSTRAINT derived_artifacts_json_chk CHECK (
    jsonb_typeof(citation_locator) = 'object'
    AND jsonb_typeof(metadata) = 'object'
    AND jsonb_typeof(warnings) = 'array'
  ),
  CONSTRAINT derived_artifacts_version_fk FOREIGN KEY (tenant_id, collection_id, evidence_id, version_id)
    REFERENCES forensic.evidence_versions (tenant_id, collection_id, evidence_id, version_id),
  CONSTRAINT derived_artifacts_run_fk FOREIGN KEY (tenant_id, collection_id, run_id)
    REFERENCES forensic.processing_runs (tenant_id, collection_id, run_id),
  CONSTRAINT derived_artifacts_storage_fk FOREIGN KEY (tenant_id, collection_id, storage_object_id)
    REFERENCES forensic.evidence_storage_objects (tenant_id, collection_id, storage_object_id),
  UNIQUE (tenant_id, collection_id, artifact_id)
);

ALTER TABLE forensic.derived_artifacts
  DROP CONSTRAINT IF EXISTS derived_artifacts_parent_fk;
ALTER TABLE forensic.derived_artifacts
  ADD CONSTRAINT derived_artifacts_parent_fk
  FOREIGN KEY (tenant_id, collection_id, parent_artifact_id)
  REFERENCES forensic.derived_artifacts (tenant_id, collection_id, artifact_id)
  DEFERRABLE INITIALLY DEFERRED;

CREATE INDEX IF NOT EXISTS derived_artifacts_evidence_idx
  ON forensic.derived_artifacts (tenant_id, collection_id, evidence_id, version_id, artifact_type);

CREATE INDEX IF NOT EXISTS derived_artifacts_run_idx
  ON forensic.derived_artifacts (tenant_id, collection_id, run_id, created_at);

CREATE TABLE IF NOT EXISTS forensic.processing_events (
  event_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id text NOT NULL,
  collection_id text NOT NULL,
  evidence_id uuid NOT NULL,
  version_id uuid NOT NULL,
  run_id uuid NOT NULL,
  artifact_id uuid,
  event_sequence bigint NOT NULL,
  event_type text NOT NULL,
  status_from text,
  status_to text,
  actor_type text NOT NULL DEFAULT 'system',
  actor_id text NOT NULL DEFAULT '',
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  occurred_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT processing_events_sequence_chk CHECK (event_sequence > 0),
  CONSTRAINT processing_events_payload_chk CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT processing_events_version_fk FOREIGN KEY (tenant_id, collection_id, evidence_id, version_id)
    REFERENCES forensic.evidence_versions (tenant_id, collection_id, evidence_id, version_id),
  CONSTRAINT processing_events_run_fk FOREIGN KEY (tenant_id, collection_id, run_id)
    REFERENCES forensic.processing_runs (tenant_id, collection_id, run_id),
  CONSTRAINT processing_events_artifact_fk FOREIGN KEY (tenant_id, collection_id, artifact_id)
    REFERENCES forensic.derived_artifacts (tenant_id, collection_id, artifact_id),
  UNIQUE (tenant_id, collection_id, run_id, event_sequence)
);

CREATE INDEX IF NOT EXISTS processing_events_evidence_idx
  ON forensic.processing_events (tenant_id, collection_id, evidence_id, occurred_at, event_id);

CREATE TABLE IF NOT EXISTS forensic.evidence_custody_events (
  custody_event_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL,
  collection_id text NOT NULL,
  evidence_id uuid NOT NULL,
  version_id uuid NOT NULL,
  source_link_id uuid,
  run_id uuid,
  artifact_id uuid,
  event_type text NOT NULL,
  actor_type text NOT NULL,
  actor_id text NOT NULL DEFAULT '',
  custody_location text,
  reason text,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  occurred_at timestamptz NOT NULL DEFAULT now(),
  previous_event_sha256 text,
  event_sha256 text NOT NULL DEFAULT repeat('0', 64),
  CONSTRAINT evidence_custody_events_payload_chk CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT evidence_custody_events_previous_hash_chk CHECK (
    previous_event_sha256 IS NULL OR previous_event_sha256 ~ '^[0-9a-f]{64}$'
  ),
  CONSTRAINT evidence_custody_events_hash_chk CHECK (event_sha256 ~ '^[0-9a-f]{64}$'),
  CONSTRAINT evidence_custody_events_version_fk FOREIGN KEY (tenant_id, collection_id, evidence_id, version_id)
    REFERENCES forensic.evidence_versions (tenant_id, collection_id, evidence_id, version_id),
  CONSTRAINT evidence_custody_events_source_fk FOREIGN KEY (tenant_id, collection_id, source_link_id)
    REFERENCES forensic.evidence_source_links (tenant_id, collection_id, source_link_id),
  CONSTRAINT evidence_custody_events_run_fk FOREIGN KEY (tenant_id, collection_id, run_id)
    REFERENCES forensic.processing_runs (tenant_id, collection_id, run_id),
  CONSTRAINT evidence_custody_events_artifact_fk FOREIGN KEY (tenant_id, collection_id, artifact_id)
    REFERENCES forensic.derived_artifacts (tenant_id, collection_id, artifact_id)
);

CREATE INDEX IF NOT EXISTS evidence_custody_events_evidence_idx
  ON forensic.evidence_custody_events (
    tenant_id, collection_id, evidence_id, occurred_at, custody_event_id
  );

CREATE OR REPLACE FUNCTION forensic.reject_append_only_mutation()
RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION '% is append-only; create a new event or version instead', TG_TABLE_NAME
    USING ERRCODE = '55000';
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION forensic.protect_storage_identity()
RETURNS trigger AS $$
BEGIN
  IF OLD.tenant_id IS DISTINCT FROM NEW.tenant_id
     OR OLD.collection_id IS DISTINCT FROM NEW.collection_id
     OR OLD.content_sha256 IS DISTINCT FROM NEW.content_sha256
     OR OLD.size_bytes IS DISTINCT FROM NEW.size_bytes
     OR OLD.storage_backend IS DISTINCT FROM NEW.storage_backend
     OR OLD.storage_uri IS DISTINCT FROM NEW.storage_uri THEN
    RAISE EXCEPTION 'storage object identity is immutable; create a new object instead'
      USING ERRCODE = '55000';
  END IF;
  NEW.updated_at := now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION forensic.protect_evidence_identity()
RETURNS trigger AS $$
BEGIN
  IF OLD.tenant_id IS DISTINCT FROM NEW.tenant_id
     OR OLD.collection_id IS DISTINCT FROM NEW.collection_id
     OR OLD.sha256 IS DISTINCT FROM NEW.sha256
     OR OLD.size_bytes IS DISTINCT FROM NEW.size_bytes
     OR OLD.raw_storage_ref IS DISTINCT FROM NEW.raw_storage_ref THEN
    RAISE EXCEPTION 'evidence byte identity is immutable; register a new version instead'
      USING ERRCODE = '55000';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION forensic.prepare_custody_event()
RETURNS trigger AS $$
DECLARE
  prior_hash text;
BEGIN
  PERFORM pg_advisory_xact_lock(hashtextextended(
    NEW.tenant_id || E'\n' || NEW.collection_id || E'\n' || NEW.evidence_id::text,
    0
  ));
  SELECT event.event_sha256
    INTO prior_hash
  FROM forensic.evidence_custody_events event
  WHERE event.tenant_id = NEW.tenant_id
    AND event.collection_id = NEW.collection_id
    AND event.evidence_id = NEW.evidence_id
    AND NOT EXISTS (
      SELECT 1
      FROM forensic.evidence_custody_events child
      WHERE child.tenant_id = NEW.tenant_id
        AND child.collection_id = NEW.collection_id
        AND child.evidence_id = NEW.evidence_id
        AND child.previous_event_sha256 = event.event_sha256
    )
  ORDER BY event.occurred_at DESC, event.custody_event_id DESC
  LIMIT 1;
  NEW.previous_event_sha256 := prior_hash;
  NEW.event_sha256 := encode(digest(convert_to(concat_ws(E'\n',
    coalesce(prior_hash, ''), NEW.custody_event_id::text, NEW.tenant_id,
    NEW.collection_id, NEW.evidence_id::text, NEW.version_id::text,
    coalesce(NEW.source_link_id::text, ''), coalesce(NEW.run_id::text, ''),
    coalesce(NEW.artifact_id::text, ''), NEW.event_type, NEW.actor_type,
    NEW.actor_id, coalesce(NEW.custody_location, ''), coalesce(NEW.reason, ''),
    NEW.occurred_at::text, NEW.payload::text
  ), 'UTF8'), 'sha256'), 'hex');
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Normalize every existing evidence item into one storage object and version.
INSERT INTO forensic.evidence_storage_objects (
  tenant_id, collection_id, content_sha256, size_bytes, storage_backend,
  storage_uri, immutability_state, write_once, verified_at,
  verification_method, metadata, created_at, updated_at
)
SELECT evidence.tenant_id, evidence.collection_id, evidence.sha256,
       evidence.size_bytes,
       CASE
         WHEN evidence.metadata->>'storage_layout_version' = 'sha256-scope-v1'
           THEN 'forensic_spool_content_addressed'
         ELSE 'legacy_spool'
       END,
       coalesce(nullif(evidence.raw_storage_ref, ''), 'unresolved://evidence/' || evidence.evidence_id::text),
       CASE
         WHEN evidence.metadata->>'storage_integrity_state' = 'verified' THEN 'verified'
         WHEN nullif(evidence.raw_storage_ref, '') IS NULL THEN 'pending'
         ELSE 'unverified'
       END,
       coalesce(evidence.metadata->>'storage_write_once' = 'true', false),
       CASE WHEN evidence.metadata->>'storage_integrity_state' = 'verified' THEN evidence.created_at END,
       CASE
         WHEN evidence.metadata->>'storage_integrity_state' = 'verified'
           THEN coalesce(nullif(evidence.metadata->>'storage_verification_method', ''), 'sha256-full-read-after-retain')
       END,
       jsonb_strip_nulls(jsonb_build_object(
         'migration_source', 'evidence_items',
         'evidence_id', evidence.evidence_id,
         'layout_version', evidence.metadata->>'storage_layout_version',
         'receipt_uri', evidence.metadata->>'storage_receipt_uri',
         'scope_key', evidence.metadata->>'storage_scope_key',
         'source_verified_at', evidence.metadata->>'storage_verified_at'
       )),
       evidence.created_at, evidence.updated_at
FROM forensic.evidence_items evidence
ON CONFLICT (tenant_id, collection_id, content_sha256, storage_uri) DO NOTHING;

INSERT INTO forensic.evidence_versions (
  version_id, tenant_id, collection_id, evidence_id, version_number,
  storage_object_id, content_sha256, size_bytes, media_type, source_name,
  version_reason, created_by, metadata, created_at
)
SELECT
  CASE
    WHEN coalesce(evidence.metadata->>'version_id', '') ~
         '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89aAbB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$'
    THEN (evidence.metadata->>'version_id')::uuid
    ELSE evidence.evidence_id
  END,
  evidence.tenant_id, evidence.collection_id, evidence.evidence_id, 1,
  storage.storage_object_id, evidence.sha256, evidence.size_bytes,
  evidence.content_type, evidence.original_filename, 'phase3_legacy_backfill',
  evidence.user_id,
  jsonb_build_object(
    'migration_source', 'evidence_items',
    'legacy_metadata_version_id', evidence.metadata->>'version_id'
  ),
  evidence.created_at
FROM forensic.evidence_items evidence
JOIN forensic.evidence_storage_objects storage
  ON storage.tenant_id = evidence.tenant_id
 AND storage.collection_id = evidence.collection_id
 AND storage.content_sha256 = evidence.sha256
 AND storage.storage_uri = coalesce(
   nullif(evidence.raw_storage_ref, ''),
   'unresolved://evidence/' || evidence.evidence_id::text
 )
WHERE NOT EXISTS (
  SELECT 1
  FROM forensic.evidence_versions existing
  WHERE existing.tenant_id = evidence.tenant_id
    AND existing.collection_id = evidence.collection_id
    AND existing.evidence_id = evidence.evidence_id
    AND existing.version_number = 1
);

UPDATE forensic.evidence_items evidence
SET current_version_id = version.version_id
FROM forensic.evidence_versions version
WHERE version.tenant_id = evidence.tenant_id
  AND version.collection_id = evidence.collection_id
  AND version.evidence_id = evidence.evidence_id
  AND version.version_number = 1
  AND evidence.current_version_id IS NULL;

ALTER TABLE forensic.evidence_items
  DROP CONSTRAINT IF EXISTS evidence_items_current_version_fk;
ALTER TABLE forensic.evidence_items
  ADD CONSTRAINT evidence_items_current_version_fk
  FOREIGN KEY (tenant_id, collection_id, evidence_id, current_version_id)
  REFERENCES forensic.evidence_versions (tenant_id, collection_id, evidence_id, version_id)
  DEFERRABLE INITIALLY DEFERRED;

INSERT INTO forensic.evidence_source_links (
  tenant_id, collection_id, evidence_id, version_id, source_kind,
  source_uri, source_name, external_source_id, relationship, metadata, created_at
)
SELECT evidence.tenant_id, evidence.collection_id, evidence.evidence_id,
       evidence.current_version_id, 'legacy',
       coalesce(nullif(evidence.raw_storage_ref, ''), 'unresolved://evidence/' || evidence.evidence_id::text),
       evidence.original_filename, evidence.evidence_id::text, 'source',
       jsonb_build_object('migration_source', 'evidence_items'), evidence.created_at
FROM forensic.evidence_items evidence
WHERE evidence.current_version_id IS NOT NULL
  AND coalesce(evidence.metadata->>'phase3_native_registration', 'false') <> 'true'
ON CONFLICT DO NOTHING;

INSERT INTO forensic.evidence_source_links (
  tenant_id, collection_id, evidence_id, version_id, source_kind,
  source_uri, source_name, external_source_id, relationship, metadata, created_at
)
SELECT assets.tenant_id, assets.collection_id, assets.evidence_id,
       evidence.current_version_id, 'knowledge_base',
       'kb://' || assets.collection_id || '/' || coalesce(nullif(assets.source_entry, ''), assets.file_id),
       assets.source_file, coalesce(nullif(assets.source_entry, ''), assets.file_id),
       'mirror',
       jsonb_build_object('migration_source', 'kb_collection_assets', 'kb_asset_id', assets.id),
       assets.created_at
FROM forensic.kb_collection_assets assets
JOIN forensic.evidence_items evidence
  ON evidence.tenant_id = assets.tenant_id
 AND evidence.collection_id = assets.collection_id
 AND evidence.evidence_id = assets.evidence_id
WHERE assets.evidence_id IS NOT NULL
  AND evidence.current_version_id IS NOT NULL
ON CONFLICT DO NOTHING;

INSERT INTO forensic.processing_runs (
  run_id, tenant_id, collection_id, evidence_id, version_id, run_kind,
  pipeline_id, adapter_id, idempotency_key, status, attempt_count,
  requested_by, requested_at, started_at, completed_at, input_sha256,
  output_manifest, error, metadata
)
SELECT jobs.id, jobs.tenant_id, jobs.collection_id, jobs.evidence_id,
       evidence.current_version_id, 'ingest', 'forensic_records_worker',
       jobs.record_type::text, 'legacy-job:' || jobs.id::text,
       CASE jobs.status::text
         WHEN 'queued' THEN 'queued'
         WHEN 'running' THEN 'running'
         WHEN 'completed' THEN 'succeeded'
         WHEN 'failed' THEN 'failed'
         WHEN 'dead_letter' THEN 'dead_letter'
         ELSE 'failed'
       END,
       jobs.attempt_count, jobs.user_id, jobs.queued_at, jobs.started_at,
       jobs.completed_at, jobs.sha256,
       jsonb_build_object(
         'total_rows', jobs.total_rows,
         'accepted_rows', jobs.accepted_rows,
         'duplicate_rows', jobs.duplicate_rows,
         'rejected_rows', jobs.rejected_rows
       ),
       CASE WHEN jobs.error_message IS NULL THEN '{}'::jsonb
            ELSE jsonb_build_object('message', jobs.error_message) END,
       jsonb_build_object('migration_source', 'records_ingest_jobs', 'legacy_metadata', jobs.metadata)
FROM forensic.records_ingest_jobs jobs
JOIN forensic.evidence_items evidence
  ON evidence.tenant_id = jobs.tenant_id
 AND evidence.collection_id = jobs.collection_id
 AND evidence.evidence_id = jobs.evidence_id
WHERE jobs.evidence_id IS NOT NULL
  AND evidence.current_version_id IS NOT NULL
ON CONFLICT (run_id) DO NOTHING;

INSERT INTO forensic.processing_events (
  tenant_id, collection_id, evidence_id, version_id, run_id,
  event_sequence, event_type, status_to, actor_type, actor_id, payload, occurred_at
)
SELECT runs.tenant_id, runs.collection_id, runs.evidence_id, runs.version_id,
       runs.run_id, 1, 'legacy_snapshot', runs.status, 'migration', '',
       jsonb_build_object('migration_source', 'records_ingest_jobs'),
       coalesce(runs.completed_at, runs.started_at, runs.requested_at)
FROM forensic.processing_runs runs
WHERE NOT EXISTS (
  SELECT 1 FROM forensic.processing_events events
  WHERE events.tenant_id = runs.tenant_id
    AND events.collection_id = runs.collection_id
    AND events.run_id = runs.run_id
);

DROP TRIGGER IF EXISTS evidence_versions_append_only ON forensic.evidence_versions;
CREATE TRIGGER evidence_versions_append_only
  BEFORE UPDATE OR DELETE ON forensic.evidence_versions
  FOR EACH ROW EXECUTE FUNCTION forensic.reject_append_only_mutation();

DROP TRIGGER IF EXISTS evidence_source_links_append_only ON forensic.evidence_source_links;
CREATE TRIGGER evidence_source_links_append_only
  BEFORE UPDATE OR DELETE ON forensic.evidence_source_links
  FOR EACH ROW EXECUTE FUNCTION forensic.reject_append_only_mutation();

DROP TRIGGER IF EXISTS processing_events_append_only ON forensic.processing_events;
CREATE TRIGGER processing_events_append_only
  BEFORE UPDATE OR DELETE ON forensic.processing_events
  FOR EACH ROW EXECUTE FUNCTION forensic.reject_append_only_mutation();

DROP TRIGGER IF EXISTS evidence_custody_events_prepare ON forensic.evidence_custody_events;
CREATE TRIGGER evidence_custody_events_prepare
  BEFORE INSERT ON forensic.evidence_custody_events
  FOR EACH ROW EXECUTE FUNCTION forensic.prepare_custody_event();

DROP TRIGGER IF EXISTS evidence_custody_events_append_only ON forensic.evidence_custody_events;
CREATE TRIGGER evidence_custody_events_append_only
  BEFORE UPDATE OR DELETE ON forensic.evidence_custody_events
  FOR EACH ROW EXECUTE FUNCTION forensic.reject_append_only_mutation();

DROP TRIGGER IF EXISTS evidence_storage_objects_protect_identity ON forensic.evidence_storage_objects;
CREATE TRIGGER evidence_storage_objects_protect_identity
  BEFORE UPDATE ON forensic.evidence_storage_objects
  FOR EACH ROW EXECUTE FUNCTION forensic.protect_storage_identity();

DROP TRIGGER IF EXISTS evidence_items_protect_identity ON forensic.evidence_items;
CREATE TRIGGER evidence_items_protect_identity
  BEFORE UPDATE ON forensic.evidence_items
  FOR EACH ROW EXECUTE FUNCTION forensic.protect_evidence_identity();

INSERT INTO forensic.evidence_custody_events (
  tenant_id, collection_id, evidence_id, version_id, event_type,
  actor_type, actor_id, reason, payload, occurred_at
)
SELECT evidence.tenant_id, evidence.collection_id, evidence.evidence_id,
       evidence.current_version_id, 'legacy_registered', 'migration', '',
       'Phase 3 normalization of an existing evidence item',
       jsonb_build_object('migration_source', 'evidence_items'), evidence.created_at
FROM forensic.evidence_items evidence
WHERE evidence.current_version_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM forensic.evidence_custody_events custody
    WHERE custody.tenant_id = evidence.tenant_id
      AND custody.collection_id = evidence.collection_id
      AND custody.evidence_id = evidence.evidence_id
  );

CREATE OR REPLACE FUNCTION forensic.sync_new_evidence_control_plane()
RETURNS trigger AS $$
DECLARE
  storage_uuid uuid;
  version_uuid uuid;
  version_text text;
  source_uuid uuid;
  resolved_uri text;
BEGIN
  resolved_uri := coalesce(nullif(NEW.raw_storage_ref, ''), 'unresolved://evidence/' || NEW.evidence_id::text);
  INSERT INTO forensic.evidence_storage_objects (
    tenant_id, collection_id, content_sha256, size_bytes, storage_backend,
    storage_uri, immutability_state, write_once, verified_at,
    verification_method, metadata
  ) VALUES (
    NEW.tenant_id, NEW.collection_id, NEW.sha256, NEW.size_bytes,
    CASE
      WHEN NEW.metadata->>'storage_layout_version' = 'sha256-scope-v1'
        THEN 'forensic_spool_content_addressed'
      ELSE 'forensic_spool'
    END,
    resolved_uri,
    CASE
      WHEN NEW.metadata->>'storage_integrity_state' = 'verified' THEN 'verified'
      WHEN nullif(NEW.raw_storage_ref, '') IS NULL THEN 'pending'
      ELSE 'unverified'
    END,
    coalesce(NEW.metadata->>'storage_write_once' = 'true', false),
    CASE WHEN NEW.metadata->>'storage_integrity_state' = 'verified' THEN NEW.created_at END,
    CASE
      WHEN NEW.metadata->>'storage_integrity_state' = 'verified'
        THEN coalesce(nullif(NEW.metadata->>'storage_verification_method', ''), 'sha256-full-read-after-retain')
    END,
    jsonb_strip_nulls(jsonb_build_object(
      'evidence_id', NEW.evidence_id,
      'layout_version', NEW.metadata->>'storage_layout_version',
      'receipt_uri', NEW.metadata->>'storage_receipt_uri',
      'scope_key', NEW.metadata->>'storage_scope_key',
      'source_verified_at', NEW.metadata->>'storage_verified_at'
    ))
  )
  ON CONFLICT (tenant_id, collection_id, content_sha256, storage_uri)
  DO UPDATE SET
    immutability_state = EXCLUDED.immutability_state,
    write_once = forensic.evidence_storage_objects.write_once OR EXCLUDED.write_once,
    verified_at = coalesce(EXCLUDED.verified_at, forensic.evidence_storage_objects.verified_at),
    verification_method = coalesce(EXCLUDED.verification_method, forensic.evidence_storage_objects.verification_method),
    metadata = forensic.evidence_storage_objects.metadata || EXCLUDED.metadata,
    updated_at = now()
  RETURNING storage_object_id INTO storage_uuid;

  version_text := coalesce(NEW.metadata->>'version_id', '');
  IF version_text ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89aAbB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$'
     AND NOT EXISTS (SELECT 1 FROM forensic.evidence_versions WHERE version_id = version_text::uuid) THEN
    version_uuid := version_text::uuid;
  ELSE
    version_uuid := gen_random_uuid();
  END IF;

  INSERT INTO forensic.evidence_versions (
    version_id, tenant_id, collection_id, evidence_id, version_number,
    storage_object_id, content_sha256, size_bytes, media_type, source_name,
    version_reason, created_by, metadata
  ) VALUES (
    version_uuid, NEW.tenant_id, NEW.collection_id, NEW.evidence_id, 1,
    storage_uuid, NEW.sha256, NEW.size_bytes, NEW.content_type,
    NEW.original_filename, 'initial_registration', NEW.user_id,
    jsonb_build_object('registration_metadata', NEW.metadata)
  );

  UPDATE forensic.evidence_items
  SET current_version_id = version_uuid,
      metadata = coalesce(metadata, '{}'::jsonb) ||
        jsonb_build_object('phase3_native_registration', true)
  WHERE evidence_id = NEW.evidence_id;

  INSERT INTO forensic.evidence_source_links (
    tenant_id, collection_id, evidence_id, version_id, source_kind,
    source_uri, source_name, external_source_id, relationship, metadata
  ) VALUES (
    NEW.tenant_id, NEW.collection_id, NEW.evidence_id, version_uuid, 'upload',
    resolved_uri, NEW.original_filename, NEW.evidence_id::text, 'source',
    jsonb_build_object('request_id', NEW.metadata->>'request_id')
  ) RETURNING source_link_id INTO source_uuid;

  INSERT INTO forensic.evidence_custody_events (
    tenant_id, collection_id, evidence_id, version_id, source_link_id,
    event_type, actor_type, actor_id, reason, payload
  ) VALUES (
    NEW.tenant_id, NEW.collection_id, NEW.evidence_id, version_uuid, source_uuid,
    'registered', 'user', NEW.user_id, 'Evidence source registered',
    jsonb_build_object('sha256', NEW.sha256, 'source_file', NEW.source_file)
  );
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION forensic.sync_kb_asset_source_link()
RETURNS trigger AS $$
DECLARE
  version_uuid uuid;
BEGIN
  IF NEW.evidence_id IS NULL THEN
    RETURN NEW;
  END IF;
  SELECT current_version_id INTO version_uuid
  FROM forensic.evidence_items
  WHERE tenant_id = NEW.tenant_id
    AND collection_id = NEW.collection_id
    AND evidence_id = NEW.evidence_id;
  IF version_uuid IS NULL THEN
    RETURN NEW;
  END IF;
  INSERT INTO forensic.evidence_source_links (
    tenant_id, collection_id, evidence_id, version_id, source_kind,
    source_uri, source_name, external_source_id, relationship, metadata
  ) VALUES (
    NEW.tenant_id, NEW.collection_id, NEW.evidence_id, version_uuid,
    'knowledge_base',
    'kb://' || NEW.collection_id || '/' || coalesce(nullif(NEW.source_entry, ''), NEW.file_id),
    NEW.source_file, coalesce(nullif(NEW.source_entry, ''), NEW.file_id),
    'mirror', jsonb_build_object('kb_asset_id', NEW.id)
  ) ON CONFLICT DO NOTHING;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION forensic.sync_ingest_job_processing_run()
RETURNS trigger AS $$
DECLARE
  version_uuid uuid;
  run_status text;
  prior_status text;
  next_sequence bigint;
BEGIN
  IF NEW.evidence_id IS NULL THEN
    RETURN NEW;
  END IF;
  SELECT current_version_id INTO version_uuid
  FROM forensic.evidence_items
  WHERE tenant_id = NEW.tenant_id
    AND collection_id = NEW.collection_id
    AND evidence_id = NEW.evidence_id;
  IF version_uuid IS NULL THEN
    RETURN NEW;
  END IF;
  run_status := CASE NEW.status::text
    WHEN 'queued' THEN 'queued'
    WHEN 'running' THEN 'running'
    WHEN 'completed' THEN 'succeeded'
    WHEN 'failed' THEN 'failed'
    WHEN 'dead_letter' THEN 'dead_letter'
    ELSE 'failed'
  END;
  IF TG_OP = 'UPDATE' THEN
    prior_status := CASE OLD.status::text
      WHEN 'queued' THEN 'queued'
      WHEN 'running' THEN 'running'
      WHEN 'completed' THEN 'succeeded'
      WHEN 'failed' THEN 'failed'
      WHEN 'dead_letter' THEN 'dead_letter'
      ELSE 'failed'
    END;
  END IF;
  INSERT INTO forensic.processing_runs (
    run_id, tenant_id, collection_id, evidence_id, version_id, run_kind,
    pipeline_id, adapter_id, idempotency_key, status, attempt_count,
    requested_by, requested_at, started_at, completed_at, input_sha256,
    output_manifest, error, metadata
  ) VALUES (
    NEW.id, NEW.tenant_id, NEW.collection_id, NEW.evidence_id, version_uuid,
    'ingest', 'forensic_records_worker', NEW.record_type::text,
    'ingest-job:' || NEW.id::text, run_status, NEW.attempt_count,
    NEW.user_id, NEW.queued_at, NEW.started_at, NEW.completed_at, NEW.sha256,
    jsonb_build_object(
      'total_rows', NEW.total_rows, 'accepted_rows', NEW.accepted_rows,
      'duplicate_rows', NEW.duplicate_rows, 'rejected_rows', NEW.rejected_rows
    ),
    CASE WHEN NEW.error_message IS NULL THEN '{}'::jsonb
         ELSE jsonb_build_object('message', NEW.error_message) END,
    jsonb_build_object('job_metadata', NEW.metadata)
  )
  ON CONFLICT (run_id) DO UPDATE SET
    status = EXCLUDED.status,
    attempt_count = EXCLUDED.attempt_count,
    started_at = EXCLUDED.started_at,
    completed_at = EXCLUDED.completed_at,
    output_manifest = EXCLUDED.output_manifest,
    error = EXCLUDED.error,
    metadata = forensic.processing_runs.metadata || EXCLUDED.metadata;

  IF TG_OP = 'INSERT' OR prior_status IS DISTINCT FROM run_status THEN
    PERFORM pg_advisory_xact_lock(hashtextextended(NEW.id::text, 0));
    SELECT coalesce(max(event_sequence), 0) + 1 INTO next_sequence
    FROM forensic.processing_events
    WHERE tenant_id = NEW.tenant_id
      AND collection_id = NEW.collection_id
      AND run_id = NEW.id;
    INSERT INTO forensic.processing_events (
      tenant_id, collection_id, evidence_id, version_id, run_id,
      event_sequence, event_type, status_from, status_to,
      actor_type, actor_id, payload
    ) VALUES (
      NEW.tenant_id, NEW.collection_id, NEW.evidence_id, version_uuid, NEW.id,
      next_sequence, CASE WHEN TG_OP = 'INSERT' THEN 'run_registered' ELSE 'status_changed' END,
      prior_status, run_status, 'worker', NEW.user_id,
      jsonb_build_object('attempt_count', NEW.attempt_count)
    );
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION forensic.sync_evidence_status_custody()
RETURNS trigger AS $$
DECLARE
  linked_run uuid;
BEGIN
  IF OLD.processing_status IS NOT DISTINCT FROM NEW.processing_status
     OR NEW.current_version_id IS NULL THEN
    RETURN NEW;
  END IF;
  SELECT run_id INTO linked_run
  FROM forensic.processing_runs
  WHERE tenant_id = NEW.tenant_id
    AND collection_id = NEW.collection_id
    AND run_id = NEW.records_batch_id;
  INSERT INTO forensic.evidence_custody_events (
    tenant_id, collection_id, evidence_id, version_id, run_id,
    event_type, actor_type, actor_id, reason, payload
  ) VALUES (
    NEW.tenant_id, NEW.collection_id, NEW.evidence_id, NEW.current_version_id,
    linked_run, 'processing_status_changed', 'system', NEW.user_id,
    'Evidence processing status changed',
    jsonb_build_object('status_from', OLD.processing_status, 'status_to', NEW.processing_status)
  );
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS evidence_items_phase3_register ON forensic.evidence_items;
CREATE TRIGGER evidence_items_phase3_register
  AFTER INSERT ON forensic.evidence_items
  FOR EACH ROW EXECUTE FUNCTION forensic.sync_new_evidence_control_plane();

DROP TRIGGER IF EXISTS evidence_items_phase3_status ON forensic.evidence_items;
CREATE TRIGGER evidence_items_phase3_status
  AFTER UPDATE OF processing_status ON forensic.evidence_items
  FOR EACH ROW EXECUTE FUNCTION forensic.sync_evidence_status_custody();

DROP TRIGGER IF EXISTS kb_collection_assets_phase3_source_insert ON forensic.kb_collection_assets;
CREATE TRIGGER kb_collection_assets_phase3_source_insert
  AFTER INSERT ON forensic.kb_collection_assets
  FOR EACH ROW EXECUTE FUNCTION forensic.sync_kb_asset_source_link();

DROP TRIGGER IF EXISTS kb_collection_assets_phase3_source_update ON forensic.kb_collection_assets;
CREATE TRIGGER kb_collection_assets_phase3_source_update
  AFTER UPDATE OF evidence_id, source_entry ON forensic.kb_collection_assets
  FOR EACH ROW EXECUTE FUNCTION forensic.sync_kb_asset_source_link();

DROP TRIGGER IF EXISTS records_ingest_jobs_phase3_run_insert ON forensic.records_ingest_jobs;
CREATE TRIGGER records_ingest_jobs_phase3_run_insert
  AFTER INSERT ON forensic.records_ingest_jobs
  FOR EACH ROW EXECUTE FUNCTION forensic.sync_ingest_job_processing_run();

DROP TRIGGER IF EXISTS records_ingest_jobs_phase3_run_update ON forensic.records_ingest_jobs;
CREATE TRIGGER records_ingest_jobs_phase3_run_update
  AFTER UPDATE OF status, attempt_count, total_rows, accepted_rows,
    duplicate_rows, rejected_rows, error_message ON forensic.records_ingest_jobs
  FOR EACH ROW EXECUTE FUNCTION forensic.sync_ingest_job_processing_run();

-- Flush deferred tenant-scoped foreign-key checks before changing table-level
-- RLS metadata in the same all-or-nothing migration transaction.
SET CONSTRAINTS ALL IMMEDIATE;

ALTER TABLE forensic.evidence_storage_objects ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.evidence_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.evidence_source_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.processing_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.processing_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.derived_artifacts ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.evidence_custody_events ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation_evidence_storage_objects ON forensic.evidence_storage_objects;
CREATE POLICY tenant_isolation_evidence_storage_objects
  ON forensic.evidence_storage_objects
  USING (tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
DROP POLICY IF EXISTS tenant_isolation_evidence_versions ON forensic.evidence_versions;
CREATE POLICY tenant_isolation_evidence_versions
  ON forensic.evidence_versions
  USING (tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
DROP POLICY IF EXISTS tenant_isolation_evidence_source_links ON forensic.evidence_source_links;
CREATE POLICY tenant_isolation_evidence_source_links
  ON forensic.evidence_source_links
  USING (tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
DROP POLICY IF EXISTS tenant_isolation_processing_runs ON forensic.processing_runs;
CREATE POLICY tenant_isolation_processing_runs
  ON forensic.processing_runs
  USING (tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
DROP POLICY IF EXISTS tenant_isolation_processing_events ON forensic.processing_events;
CREATE POLICY tenant_isolation_processing_events
  ON forensic.processing_events
  USING (tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
DROP POLICY IF EXISTS tenant_isolation_derived_artifacts ON forensic.derived_artifacts;
CREATE POLICY tenant_isolation_derived_artifacts
  ON forensic.derived_artifacts
  USING (tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
DROP POLICY IF EXISTS tenant_isolation_evidence_custody_events ON forensic.evidence_custody_events;
CREATE POLICY tenant_isolation_evidence_custody_events
  ON forensic.evidence_custody_events
  USING (tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

COMMIT;
