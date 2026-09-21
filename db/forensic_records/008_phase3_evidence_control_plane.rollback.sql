-- Destructive rollback for 008_phase3_evidence_control_plane.sql.
-- Run only from a verified backup with ON_ERROR_STOP enabled and an explicit:
--   SET app.phase3_allow_destructive_rollback = 'true';

BEGIN;

DO $$
BEGIN
  IF current_setting('app.phase3_allow_destructive_rollback', true) IS DISTINCT FROM 'true' THEN
    RAISE EXCEPTION 'Phase 3 rollback is destructive; set app.phase3_allow_destructive_rollback=true explicitly'
      USING ERRCODE = '55000';
  END IF;
END $$;

DROP TRIGGER IF EXISTS records_ingest_jobs_phase3_run_update ON forensic.records_ingest_jobs;
DROP TRIGGER IF EXISTS records_ingest_jobs_phase3_run_insert ON forensic.records_ingest_jobs;
DROP TRIGGER IF EXISTS kb_collection_assets_phase3_source_update ON forensic.kb_collection_assets;
DROP TRIGGER IF EXISTS kb_collection_assets_phase3_source_insert ON forensic.kb_collection_assets;
DROP TRIGGER IF EXISTS evidence_items_phase3_status ON forensic.evidence_items;
DROP TRIGGER IF EXISTS evidence_items_phase3_register ON forensic.evidence_items;
DROP TRIGGER IF EXISTS evidence_items_protect_identity ON forensic.evidence_items;

ALTER TABLE forensic.evidence_items
  DROP CONSTRAINT IF EXISTS evidence_items_current_version_fk;
ALTER TABLE forensic.evidence_items
  DROP COLUMN IF EXISTS current_version_id;

DROP TABLE IF EXISTS forensic.evidence_custody_events;
DROP TABLE IF EXISTS forensic.processing_events;
DROP TABLE IF EXISTS forensic.derived_artifacts;
DROP TABLE IF EXISTS forensic.processing_runs;
DROP TABLE IF EXISTS forensic.evidence_source_links;
DROP TABLE IF EXISTS forensic.evidence_versions;
DROP TABLE IF EXISTS forensic.evidence_storage_objects;

DROP FUNCTION IF EXISTS forensic.sync_evidence_status_custody();
DROP FUNCTION IF EXISTS forensic.sync_ingest_job_processing_run();
DROP FUNCTION IF EXISTS forensic.sync_kb_asset_source_link();
DROP FUNCTION IF EXISTS forensic.sync_new_evidence_control_plane();
DROP FUNCTION IF EXISTS forensic.prepare_custody_event();
DROP FUNCTION IF EXISTS forensic.protect_evidence_identity();
DROP FUNCTION IF EXISTS forensic.protect_storage_identity();
DROP FUNCTION IF EXISTS forensic.reject_append_only_mutation();

DROP INDEX IF EXISTS forensic.evidence_items_tenant_identity_idx;

COMMIT;
