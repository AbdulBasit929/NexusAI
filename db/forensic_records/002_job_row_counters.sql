-- Add job-level row counters for operational status screens and CLI checks.
-- This is safe to apply to databases initialized before these columns existed.

ALTER TABLE forensic.records_ingest_jobs
  ADD COLUMN IF NOT EXISTS total_rows bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS accepted_rows bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS duplicate_rows bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS rejected_rows bigint NOT NULL DEFAULT 0;

UPDATE forensic.records_ingest_jobs jobs
SET
  total_rows = metadata.total_rows,
  accepted_rows = metadata.inserted_rows,
  duplicate_rows = metadata.duplicate_rows,
  rejected_rows = metadata.rejected_rows
FROM forensic.kb_active_metadata metadata
WHERE jobs.tenant_id = metadata.tenant_id
  AND jobs.collection_id = metadata.collection_id
  AND jobs.file_id = metadata.file_id
  AND jobs.id = metadata.batch_id
  AND jobs.total_rows = 0;
