-- Backfill missing forensic.kb_collection_assets rows from completed ingest jobs.
-- Usage:
--   psql -v tenant_id=default -v collection_id=records-demo -f scripts/forensic_repair_assets.sql

WITH candidates AS (
  SELECT jobs.*,
         metadata.normalized_schema,
         metadata.quality_report AS metadata_quality_report
  FROM forensic.records_ingest_jobs jobs
  LEFT JOIN forensic.kb_collection_assets assets
    ON assets.tenant_id = jobs.tenant_id
   AND assets.collection_id = jobs.collection_id
   AND assets.file_id = jobs.file_id
   AND assets.sha256 = jobs.sha256
  LEFT JOIN forensic.kb_active_metadata metadata
    ON metadata.tenant_id = jobs.tenant_id
   AND metadata.collection_id = jobs.collection_id
   AND metadata.file_id = jobs.file_id
   AND metadata.batch_id = jobs.id
  WHERE jobs.tenant_id = :'tenant_id'
    AND jobs.collection_id = :'collection_id'
    AND jobs.status = 'completed'
    AND assets.id IS NULL
)
INSERT INTO forensic.kb_collection_assets (
  tenant_id, user_id, collection_id, file_id, batch_id, source_file,
  source_entry, sha256, detected_record_type, requested_record_type, storage_mode,
  rag_status, structured_status, content_type, size_bytes, headers,
  routing_decision, quality_report
)
SELECT
  tenant_id,
  user_id,
  collection_id,
  file_id,
  id,
  source_file,
  nullif(metadata->>'kb_source_entry', ''),
  sha256,
  record_type,
  record_type,
  CASE WHEN nullif(metadata->>'kb_source_entry', '') IS NULL THEN 'records_only' ELSE 'hybrid' END,
  CASE
    WHEN metadata->>'kb_mirror_status' = 'mirrored' THEN 'mirrored'
    WHEN metadata->>'kb_mirror_status' = 'failed' THEN 'failed'
    ELSE 'skipped'
  END,
  'completed',
  nullif(metadata->>'content_type', ''),
  CASE
    WHEN metadata->>'size_bytes' ~ '^[0-9]+$' THEN (metadata->>'size_bytes')::bigint
    ELSE NULL
  END,
  '[]'::jsonb,
  jsonb_build_object(
    'storage_mode', CASE WHEN nullif(metadata->>'kb_source_entry', '') IS NULL THEN 'records_only' ELSE 'hybrid' END,
    'requested_record_type', record_type::text,
    'detected_record_type', record_type::text,
    'structured_store', CASE WHEN record_type = 'cdr' THEN 'cdr_records' ELSE 'generic_records' END,
    'kb_collection', collection_id,
    'routing_reason', 'asset_repair_from_completed_ingest_job'
  ),
  jsonb_build_object(
    'repair_source', 'records_ingest_jobs',
    'repaired_at', now(),
    'total_rows', total_rows,
    'accepted_rows', accepted_rows,
    'duplicate_rows', duplicate_rows,
    'rejected_rows', rejected_rows,
    'normalized_schema', coalesce(normalized_schema, '{}'::jsonb),
    'metadata_quality_report', coalesce(metadata_quality_report, '{}'::jsonb)
  )
FROM candidates
ON CONFLICT (tenant_id, collection_id, file_id, sha256)
DO UPDATE SET
  batch_id=EXCLUDED.batch_id,
  detected_record_type=EXCLUDED.detected_record_type,
  requested_record_type=EXCLUDED.requested_record_type,
  storage_mode=EXCLUDED.storage_mode,
  rag_status=EXCLUDED.rag_status,
  structured_status=EXCLUDED.structured_status,
  routing_decision=EXCLUDED.routing_decision,
  quality_report=forensic.kb_collection_assets.quality_report || EXCLUDED.quality_report,
  updated_at=now();
