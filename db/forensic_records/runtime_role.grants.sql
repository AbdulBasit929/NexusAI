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

-- Typed families, reference tables, the entity and relationship layer and embeddings (migrations 012 to 015): append-only
-- INSERT on every table that exists, column-level UPDATE only where an analyst decision must be recorded, and no access at
-- all to the identity table. Applies only to tables that exist, so the file stays valid before those migrations.
DO $$
DECLARE
  t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'layout_versions', 'comm_event', 'data_session', 'app_usage', 'plate_sighting', 'subscriber_registration',
    'cell_site', 'camera_site', 'device_tac', 'ip_range_reference',
    'derivation', 'entity', 'entity_mention', 'relationship', 'relationship_evidence', 'ip_assignment',
    'content_embedding'
  ]
  LOOP
    IF to_regclass('forensic.' || t) IS NOT NULL THEN
      EXECUTE format('GRANT INSERT ON forensic.%I TO forensic_runtime', t);
    END IF;
  END LOOP;

  IF to_regclass('forensic.entity') IS NOT NULL THEN
    GRANT UPDATE (display_label, first_seen, last_seen, attributes, merged_into, updated_at)
      ON forensic.entity TO forensic_runtime;
  END IF;
  IF to_regclass('forensic.relationship') IS NOT NULL THEN
    GRANT UPDATE (first_seen, last_seen, evidence_count, features, score, review_state, reviewed_by, reviewed_at, updated_at)
      ON forensic.relationship TO forensic_runtime;
  END IF;
  IF to_regclass('forensic.subscriber_identity') IS NOT NULL THEN
    REVOKE ALL ON forensic.subscriber_identity FROM forensic_runtime;
  END IF;
END
$$;
