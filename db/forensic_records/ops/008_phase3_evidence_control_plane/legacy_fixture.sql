-- Disposable-database fixture: install after migrations 001-007 and before 008.
-- It represents Phase 2 rows that must survive and normalize during Phase 3.

INSERT INTO forensic.evidence_items (
  evidence_id, tenant_id, collection_id, case_id, user_id,
  original_filename, source_file, content_type, extension, size_bytes,
  sha256, modality, detected_type, classifier_confidence,
  processing_route, processing_status, raw_storage_ref, metadata
) VALUES (
  '31000000-0000-4000-8000-000000000001',
  'phase3-legacy', 'phase3-legacy-collection', 'PK-SYNTHETIC-LEGACY',
  'phase3-legacy-user', 'legacy-cdr.csv', 'legacy-cdr.csv',
  'text/csv', 'csv', 256, repeat('b', 64), 'structured_records',
  'cdr', 1, 'forensic_records_worker', 'completed',
  'immutable://phase3-legacy/sha256/' || repeat('b', 64),
  jsonb_build_object('request_id', 'phase3-legacy-request')
);

INSERT INTO forensic.kb_collection_assets (
  id, tenant_id, user_id, collection_id, file_id, source_file,
  source_entry, sha256, detected_record_type, requested_record_type,
  storage_mode, rag_status, structured_status, content_type, size_bytes,
  evidence_id, headers, routing_decision, quality_report
) VALUES
  (
    '31000000-0000-4000-8000-000000000002',
    'phase3-legacy', 'phase3-legacy-user', 'phase3-legacy-collection',
    'legacy-cdr-file', 'legacy-cdr.csv', 'legacy-cdr.csv', repeat('b', 64),
    'cdr', 'cdr', 'hybrid', 'mirrored', 'completed', 'text/csv', 256,
    '31000000-0000-4000-8000-000000000001', '[]'::jsonb, '{}'::jsonb, '{}'::jsonb
  ),
  (
    '31000000-0000-4000-8000-000000000003',
    'phase3-legacy', 'phase3-legacy-user', 'phase3-legacy-collection',
    'legacy-cdr-sidecar', 'legacy-cdr.csv', 'legacy-cdr.csv.metadata.json', repeat('c', 64),
    'generic', 'generic', 'rag_only', 'mirrored', 'skipped', 'application/json', 64,
    '31000000-0000-4000-8000-000000000001', '[]'::jsonb, '{}'::jsonb, '{}'::jsonb
  );

INSERT INTO forensic.records_ingest_jobs (
  id, tenant_id, user_id, collection_id, file_id, source_file,
  spool_path, sha256, record_type, status, attempt_count,
  total_rows, accepted_rows, duplicate_rows, rejected_rows,
  queued_at, started_at, completed_at, evidence_id, metadata
) VALUES (
  '31000000-0000-4000-8000-000000000004',
  'phase3-legacy', 'phase3-legacy-user', 'phase3-legacy-collection',
  'legacy-cdr-file', 'legacy-cdr.csv', '/isolated/legacy-cdr.csv',
  repeat('b', 64), 'cdr', 'completed', 1, 3, 2, 1, 0,
  now() - interval '2 minutes', now() - interval '1 minute', now(),
  '31000000-0000-4000-8000-000000000001',
  jsonb_build_object('request_id', 'phase3-legacy-request')
);

SELECT 'phase3_legacy_fixture_installed' AS result;
