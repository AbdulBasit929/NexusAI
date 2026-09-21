-- Read-only acceptance checks for the Phase 3 control-plane migration.

DO $$
DECLARE
  relation_name text;
  missing_count bigint;
BEGIN
  FOREACH relation_name IN ARRAY ARRAY[
    'forensic.evidence_storage_objects',
    'forensic.evidence_versions',
    'forensic.evidence_source_links',
    'forensic.processing_runs',
    'forensic.processing_events',
    'forensic.derived_artifacts',
    'forensic.evidence_custody_events'
  ] LOOP
    IF to_regclass(relation_name) IS NULL THEN
      RAISE EXCEPTION 'missing Phase 3 relation %', relation_name;
    END IF;
  END LOOP;

  SELECT count(*) INTO missing_count
  FROM forensic.evidence_items
  WHERE current_version_id IS NULL;
  IF missing_count <> 0 THEN
    RAISE EXCEPTION '% evidence item(s) have no current version', missing_count;
  END IF;

  SELECT count(*) INTO missing_count
  FROM forensic.evidence_items evidence
  LEFT JOIN forensic.evidence_versions version
    ON version.tenant_id = evidence.tenant_id
   AND version.collection_id = evidence.collection_id
   AND version.evidence_id = evidence.evidence_id
   AND version.version_id = evidence.current_version_id
  WHERE version.version_id IS NULL;
  IF missing_count <> 0 THEN
    RAISE EXCEPTION '% evidence current-version pointer(s) are invalid', missing_count;
  END IF;

  SELECT count(*) INTO missing_count
  FROM forensic.evidence_storage_objects
  WHERE storage_backend = 'forensic_spool_content_addressed'
    AND (
      storage_uri !~ '^forensic-spool://sha256-scope-v1/[0-9a-f]{64}/[0-9a-f]{64}$'
      OR split_part(storage_uri, '/', 5) <> content_sha256
      OR immutability_state <> 'verified'
      OR NOT write_once
      OR verified_at IS NULL
      OR verification_method <> 'sha256-full-read-after-retain'
    );
  IF missing_count <> 0 THEN
    RAISE EXCEPTION '% content-addressed storage object(s) are not verified write-once objects', missing_count;
  END IF;

  SELECT count(*) INTO missing_count
  FROM forensic.kb_collection_assets asset
  WHERE asset.evidence_id IS NOT NULL
    AND NOT EXISTS (
      SELECT 1 FROM forensic.evidence_source_links source
      WHERE source.tenant_id = asset.tenant_id
        AND source.collection_id = asset.collection_id
        AND source.evidence_id = asset.evidence_id
        AND source.source_kind = 'knowledge_base'
        AND source.external_source_id = coalesce(nullif(asset.source_entry, ''), asset.file_id)
    );
  IF missing_count <> 0 THEN
    RAISE EXCEPTION '% KB asset(s) have no normalized source link', missing_count;
  END IF;

  SELECT count(*) INTO missing_count
  FROM forensic.records_ingest_jobs job
  WHERE job.evidence_id IS NOT NULL
    AND NOT EXISTS (
      SELECT 1 FROM forensic.processing_runs run
      WHERE run.run_id = job.id
        AND run.tenant_id = job.tenant_id
        AND run.collection_id = job.collection_id
    );
  IF missing_count <> 0 THEN
    RAISE EXCEPTION '% ingest job(s) have no processing run', missing_count;
  END IF;

  SELECT count(*) INTO missing_count
  FROM forensic.evidence_custody_events
  WHERE event_sha256 !~ '^[0-9a-f]{64}$';
  IF missing_count <> 0 THEN
    RAISE EXCEPTION '% custody event(s) have invalid hashes', missing_count;
  END IF;

  SELECT count(*) INTO missing_count
  FROM pg_policies
  WHERE schemaname = 'forensic'
    AND tablename IN (
      'evidence_storage_objects', 'evidence_versions', 'evidence_source_links',
      'processing_runs', 'processing_events', 'derived_artifacts',
      'evidence_custody_events'
    );
  IF missing_count <> 7 THEN
    RAISE EXCEPTION 'expected 7 Phase 3 tenant policies, found %', missing_count;
  END IF;
END $$;

SELECT
  (SELECT count(*) FROM forensic.evidence_items) AS evidence_items,
  (SELECT count(*) FROM forensic.evidence_storage_objects) AS storage_objects,
  (SELECT count(*) FROM forensic.evidence_versions) AS evidence_versions,
  (SELECT count(*) FROM forensic.evidence_source_links) AS source_links,
  (SELECT count(*) FROM forensic.processing_runs) AS processing_runs,
  (SELECT count(*) FROM forensic.processing_events) AS processing_events,
  (SELECT count(*) FROM forensic.derived_artifacts) AS derived_artifacts,
  (SELECT count(*) FROM forensic.evidence_custody_events) AS custody_events;
