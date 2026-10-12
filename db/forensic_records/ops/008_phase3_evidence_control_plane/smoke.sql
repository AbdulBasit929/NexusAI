-- Stateful smoke checks for an isolated disposable PostgreSQL database only.

SET app.tenant_id = 'phase3-smoke';

INSERT INTO forensic.evidence_items (
  evidence_id, tenant_id, collection_id, case_id, user_id,
  original_filename, source_file, content_type, extension, size_bytes,
  sha256, modality, detected_type, classifier_confidence,
  processing_route, processing_status, raw_storage_ref, metadata
) VALUES (
  '30000000-0000-4000-8000-000000000001',
  'phase3-smoke', 'phase3-control-plane-smoke', 'PK-SYNTHETIC-PHASE3',
  'phase3-test-user', 'phase3-smoke.jsonl', 'phase3-smoke.jsonl',
  'application/x-ndjson', 'jsonl', 128,
  repeat('a', 64), 'structured_records', 'generic', 1,
  'forensic_records_worker', 'queued',
  'forensic-spool://sha256-scope-v1/2dbfa9b88e4316f8cac0ee0094b8d25e65651bcf2ecc1fc385edfb255d07bb42/' || repeat('a', 64),
  jsonb_build_object(
    'version_id', '30000000-0000-4000-8000-000000000002',
    'request_id', 'phase3-smoke-request',
    'storage_layout_version', 'sha256-scope-v1',
    'storage_scope_key', '2dbfa9b88e4316f8cac0ee0094b8d25e65651bcf2ecc1fc385edfb255d07bb42',
    'storage_receipt_uri', 'forensic-spool-receipt://sha256-scope-v1/2dbfa9b88e4316f8cac0ee0094b8d25e65651bcf2ecc1fc385edfb255d07bb42/' || repeat('a', 64),
    'storage_integrity_state', 'verified',
    'storage_write_once', 'true',
    'storage_verification_method', 'sha256-full-read-after-retain',
    'storage_verified_at', '2026-07-29T03:00:00Z'
  )
);

INSERT INTO forensic.kb_collection_assets (
  id, tenant_id, user_id, collection_id, file_id, source_file,
  source_entry, sha256, detected_record_type, requested_record_type,
  storage_mode, rag_status, structured_status, content_type, size_bytes,
  evidence_id, headers, routing_decision, quality_report
) VALUES (
  '30000000-0000-4000-8000-000000000003',
  'phase3-smoke', 'phase3-test-user', 'phase3-control-plane-smoke',
  'phase3-smoke-file', 'phase3-smoke.jsonl',
  'phase3-smoke-entry/phase3-smoke.jsonl', repeat('a', 64),
  'generic', 'generic', 'hybrid', 'mirrored', 'pending',
  'application/x-ndjson', 128,
  '30000000-0000-4000-8000-000000000001', '[]'::jsonb,
  '{}'::jsonb, '{}'::jsonb
);

INSERT INTO forensic.records_ingest_jobs (
  id, tenant_id, user_id, collection_id, file_id, source_file,
  spool_path, sha256, record_type, status, evidence_id, metadata
) VALUES (
  '30000000-0000-4000-8000-000000000004',
  'phase3-smoke', 'phase3-test-user', 'phase3-control-plane-smoke',
  'phase3-smoke-file', 'phase3-smoke.jsonl',
  '/isolated/phase3-smoke.jsonl', repeat('a', 64), 'generic', 'queued',
  '30000000-0000-4000-8000-000000000001',
  jsonb_build_object('request_id', 'phase3-smoke-request')
);

UPDATE forensic.records_ingest_jobs
SET status = 'running', attempt_count = 1, started_at = now()
WHERE id = '30000000-0000-4000-8000-000000000004';

UPDATE forensic.evidence_items
SET processing_status = 'processing',
    records_batch_id = '30000000-0000-4000-8000-000000000004'
WHERE evidence_id = '30000000-0000-4000-8000-000000000001';

UPDATE forensic.records_ingest_jobs
SET status = 'completed', completed_at = now(), total_rows = 2,
    accepted_rows = 1, duplicate_rows = 1, rejected_rows = 0
WHERE id = '30000000-0000-4000-8000-000000000004';

UPDATE forensic.evidence_items
SET processing_status = 'completed'
WHERE evidence_id = '30000000-0000-4000-8000-000000000001';

DO $$
DECLARE
  actual bigint;
  protected boolean;
BEGIN
  SELECT count(*) INTO actual FROM forensic.evidence_storage_objects
  WHERE tenant_id = 'phase3-smoke';
  IF actual <> 1 THEN RAISE EXCEPTION 'expected 1 storage object, found %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.evidence_storage_objects
  WHERE tenant_id = 'phase3-smoke'
    AND storage_backend = 'forensic_spool_content_addressed'
    AND immutability_state = 'verified'
    AND write_once
    AND verification_method = 'sha256-full-read-after-retain'
    AND verified_at IS NOT NULL;
  IF actual <> 1 THEN RAISE EXCEPTION 'expected one verified write-once content-addressed object, found %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.evidence_versions
  WHERE tenant_id = 'phase3-smoke';
  IF actual <> 1 THEN RAISE EXCEPTION 'expected 1 evidence version, found %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.evidence_source_links
  WHERE tenant_id = 'phase3-smoke';
  IF actual <> 2 THEN RAISE EXCEPTION 'expected 2 source links, found %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.processing_runs
  WHERE tenant_id = 'phase3-smoke' AND status = 'succeeded';
  IF actual <> 1 THEN RAISE EXCEPTION 'expected 1 successful processing run, found %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.processing_events
  WHERE tenant_id = 'phase3-smoke';
  IF actual <> 3 THEN RAISE EXCEPTION 'expected 3 processing events, found %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.evidence_custody_events
  WHERE tenant_id = 'phase3-smoke';
  IF actual <> 3 THEN RAISE EXCEPTION 'expected 3 custody events, found %', actual; END IF;

  SELECT count(*) INTO actual
  FROM forensic.evidence_custody_events current_event
  WHERE current_event.tenant_id = 'phase3-smoke'
    AND current_event.event_sha256 ~ '^[0-9a-f]{64}$'
    AND (
      current_event.previous_event_sha256 IS NULL
      OR EXISTS (
        SELECT 1 FROM forensic.evidence_custody_events prior_event
        WHERE prior_event.tenant_id = current_event.tenant_id
          AND prior_event.collection_id = current_event.collection_id
          AND prior_event.evidence_id = current_event.evidence_id
          AND prior_event.event_sha256 = current_event.previous_event_sha256
      )
    );
  IF actual <> 3 THEN RAISE EXCEPTION 'custody hash chain is incomplete'; END IF;

  protected := false;
  BEGIN
    UPDATE forensic.processing_events SET event_type = 'tampered'
    WHERE tenant_id = 'phase3-smoke';
  EXCEPTION WHEN SQLSTATE '55000' THEN
    protected := true;
  END;
  IF NOT protected THEN RAISE EXCEPTION 'processing events were mutable'; END IF;

  protected := false;
  BEGIN
    UPDATE forensic.evidence_versions SET content_sha256 = repeat('b', 64)
    WHERE tenant_id = 'phase3-smoke';
  EXCEPTION WHEN SQLSTATE '55000' THEN
    protected := true;
  END;
  IF NOT protected THEN RAISE EXCEPTION 'evidence versions were mutable'; END IF;

  protected := false;
  BEGIN
    UPDATE forensic.evidence_items SET sha256 = repeat('b', 64)
    WHERE tenant_id = 'phase3-smoke';
  EXCEPTION WHEN SQLSTATE '55000' THEN
    protected := true;
  END;
  IF NOT protected THEN RAISE EXCEPTION 'evidence byte identity was mutable'; END IF;
END $$;

SELECT 'phase3_control_plane_smoke_passed' AS result;
