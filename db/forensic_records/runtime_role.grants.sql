-- Provision the fixed, non-owner application role. Set its password separately;
-- never commit credentials to this file or pass them on the command line.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    CREATE ROLE forensic_runtime LOGIN;
  END IF;
END
$$;

ALTER ROLE forensic_runtime
  LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOINHERIT NOBYPASSRLS;

REVOKE CREATE ON SCHEMA forensic FROM PUBLIC;
REVOKE ALL PRIVILEGES ON SCHEMA forensic FROM forensic_runtime;
REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA forensic FROM forensic_runtime;
REVOKE ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA forensic FROM forensic_runtime;

GRANT CONNECT, TEMPORARY ON DATABASE localrecall TO forensic_runtime;
GRANT USAGE ON SCHEMA forensic TO forensic_runtime;
GRANT SELECT ON ALL TABLES IN SCHEMA forensic TO forensic_runtime;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA forensic TO forensic_runtime;

GRANT INSERT ON
  forensic.evidence_items,
  forensic.evidence_storage_objects,
  forensic.evidence_versions,
  forensic.evidence_source_links,
  forensic.processing_runs,
  forensic.processing_events,
  forensic.derived_artifacts,
  forensic.evidence_custody_events,
  forensic.records_ingest_jobs,
  forensic.records_ingest_errors,
  forensic.records_audit_log,
  forensic.cdr_records,
  forensic.generic_records,
  forensic.records,
  forensic.record_entities,
  forensic.kb_active_metadata,
  forensic.kb_collection_assets
TO forensic_runtime;

GRANT UPDATE ON
  forensic.evidence_items,
  forensic.evidence_storage_objects,
  forensic.processing_runs,
  forensic.records_ingest_jobs,
  forensic.kb_active_metadata,
  forensic.kb_collection_assets
TO forensic_runtime;
