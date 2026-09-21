-- Disposable-database non-owner RLS acceptance. Run after the legacy fixture,
-- migration 008, and native smoke. The transaction rolls back its role and row.

BEGIN;

CREATE ROLE phase3_rls_test NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
GRANT USAGE ON SCHEMA forensic TO phase3_rls_test;
GRANT SELECT, INSERT ON
  forensic.evidence_storage_objects,
  forensic.evidence_versions,
  forensic.evidence_source_links,
  forensic.processing_runs,
  forensic.processing_events,
  forensic.derived_artifacts,
  forensic.evidence_custody_events
TO phase3_rls_test;

SET ROLE phase3_rls_test;
SET app.tenant_id = 'phase3-smoke';

DO $$
DECLARE
  actual bigint;
  blocked boolean := false;
BEGIN
  SELECT count(*) INTO actual FROM forensic.evidence_storage_objects;
  IF actual <> 1 THEN RAISE EXCEPTION 'RLS expected 1 tenant storage object, found %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.evidence_versions;
  IF actual <> 1 THEN RAISE EXCEPTION 'RLS expected 1 tenant version, found %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.evidence_source_links;
  IF actual <> 2 THEN RAISE EXCEPTION 'RLS expected 2 tenant source links, found %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.processing_runs;
  IF actual <> 1 THEN RAISE EXCEPTION 'RLS expected 1 tenant processing run, found %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.processing_events;
  IF actual <> 3 THEN RAISE EXCEPTION 'RLS expected 3 tenant processing events, found %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.derived_artifacts;
  IF actual <> 0 THEN RAISE EXCEPTION 'RLS expected 0 tenant derived artifacts, found %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.evidence_custody_events;
  IF actual <> 3 THEN RAISE EXCEPTION 'RLS expected 3 tenant custody events, found %', actual; END IF;

  BEGIN
    INSERT INTO forensic.evidence_source_links (
      tenant_id, collection_id, evidence_id, version_id, source_kind,
      source_uri, source_name, external_source_id, relationship
    ) VALUES (
      'phase3-legacy', 'phase3-legacy-collection',
      '31000000-0000-4000-8000-000000000001',
      '31000000-0000-4000-8000-000000000001',
      'external', 'test://cross-tenant', 'forbidden', 'forbidden', 'import'
    );
  EXCEPTION WHEN insufficient_privilege THEN
    blocked := true;
  END;
  IF NOT blocked THEN RAISE EXCEPTION 'RLS allowed a cross-tenant insert'; END IF;

  INSERT INTO forensic.evidence_source_links (
    tenant_id, collection_id, evidence_id, version_id, source_kind,
    source_uri, source_name, external_source_id, relationship
  ) VALUES (
    'phase3-smoke', 'phase3-control-plane-smoke',
    '30000000-0000-4000-8000-000000000001',
    '30000000-0000-4000-8000-000000000002',
    'external', 'test://same-tenant', 'allowed', 'allowed', 'import'
  );

  SELECT count(*) INTO actual FROM forensic.evidence_source_links;
  IF actual <> 3 THEN RAISE EXCEPTION 'RLS same-tenant insert did not remain visible'; END IF;
END $$;

SELECT 'phase3_non_owner_rls_smoke_passed' AS result;

RESET ROLE;
ROLLBACK;
