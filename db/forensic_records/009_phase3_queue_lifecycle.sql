-- Phase 3 durable queue lifecycle: transactional publication outbox, worker
-- leases, bounded retry/DLQ state and append-only reprocessing jobs.
BEGIN;

ALTER TABLE forensic.records_ingest_jobs
  ADD COLUMN IF NOT EXISTS queue_message_id text,
  ADD COLUMN IF NOT EXISTS queue_published_at timestamptz,
  ADD COLUMN IF NOT EXISTS queue_publish_attempts integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS queue_publish_next_at timestamptz,
  ADD COLUMN IF NOT EXISTS queue_publish_error text,
  ADD COLUMN IF NOT EXISTS queue_publish_lease_token uuid,
  ADD COLUMN IF NOT EXISTS queue_publish_lease_expires_at timestamptz,
  ADD COLUMN IF NOT EXISTS worker_lease_token uuid,
  ADD COLUMN IF NOT EXISTS worker_lease_owner text,
  ADD COLUMN IF NOT EXISTS worker_lease_expires_at timestamptz,
  ADD COLUMN IF NOT EXISTS next_attempt_at timestamptz,
  ADD COLUMN IF NOT EXISTS last_error_class text,
  ADD COLUMN IF NOT EXISTS dead_lettered_at timestamptz,
  ADD COLUMN IF NOT EXISTS acknowledged_at timestamptz,
  ADD COLUMN IF NOT EXISTS reprocess_of_job_id uuid,
  ADD COLUMN IF NOT EXISTS reprocess_generation integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS reprocess_request_key text;

UPDATE forensic.records_ingest_jobs
SET queue_message_id = 'ingest-job:' || id::text
WHERE queue_message_id IS NULL;

ALTER TABLE forensic.records_ingest_jobs
  ALTER COLUMN queue_message_id SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'records_ingest_jobs_reprocess_parent_fk'
      AND conrelid = 'forensic.records_ingest_jobs'::regclass
  ) THEN
    ALTER TABLE forensic.records_ingest_jobs
      ADD CONSTRAINT records_ingest_jobs_reprocess_parent_fk
      FOREIGN KEY (reprocess_of_job_id)
      REFERENCES forensic.records_ingest_jobs(id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'records_ingest_jobs_queue_counters_chk'
      AND conrelid = 'forensic.records_ingest_jobs'::regclass
  ) THEN
    ALTER TABLE forensic.records_ingest_jobs
      ADD CONSTRAINT records_ingest_jobs_queue_counters_chk CHECK (
        queue_publish_attempts >= 0
        AND reprocess_generation >= 0
      );
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'records_ingest_jobs_reprocess_identity_chk'
      AND conrelid = 'forensic.records_ingest_jobs'::regclass
  ) THEN
    ALTER TABLE forensic.records_ingest_jobs
      ADD CONSTRAINT records_ingest_jobs_reprocess_identity_chk CHECK (
        (reprocess_generation = 0 AND reprocess_of_job_id IS NULL)
        OR (reprocess_generation > 0 AND reprocess_of_job_id IS NOT NULL)
      );
  END IF;
END
$$;

CREATE INDEX IF NOT EXISTS records_ingest_jobs_publish_due_idx
  ON forensic.records_ingest_jobs (queue_publish_next_at, queued_at)
  WHERE queue_published_at IS NULL AND status = 'queued';

CREATE INDEX IF NOT EXISTS records_ingest_jobs_worker_due_idx
  ON forensic.records_ingest_jobs (next_attempt_at, queued_at)
  WHERE status IN ('queued', 'failed', 'running');

CREATE INDEX IF NOT EXISTS records_ingest_jobs_worker_lease_idx
  ON forensic.records_ingest_jobs (worker_lease_expires_at)
  WHERE worker_lease_token IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS records_ingest_jobs_reprocess_request_uidx
  ON forensic.records_ingest_jobs (tenant_id, collection_id, reprocess_request_key)
  WHERE reprocess_request_key IS NOT NULL;

CREATE OR REPLACE FUNCTION forensic.protect_ingest_job_history()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF OLD.tenant_id IS DISTINCT FROM NEW.tenant_id
     OR OLD.collection_id IS DISTINCT FROM NEW.collection_id
     OR OLD.file_id IS DISTINCT FROM NEW.file_id
     OR OLD.source_file IS DISTINCT FROM NEW.source_file
     OR OLD.spool_path IS DISTINCT FROM NEW.spool_path
     OR OLD.sha256 IS DISTINCT FROM NEW.sha256
     OR OLD.record_type IS DISTINCT FROM NEW.record_type
     OR OLD.evidence_id IS DISTINCT FROM NEW.evidence_id
     OR OLD.queue_message_id IS DISTINCT FROM NEW.queue_message_id
     OR OLD.reprocess_of_job_id IS DISTINCT FROM NEW.reprocess_of_job_id
     OR OLD.reprocess_generation IS DISTINCT FROM NEW.reprocess_generation
     OR OLD.reprocess_request_key IS DISTINCT FROM NEW.reprocess_request_key THEN
    RAISE EXCEPTION 'ingest job evidence and reprocessing identity are immutable';
  END IF;

  IF NEW.attempt_count < OLD.attempt_count THEN
    RAISE EXCEPTION 'ingest job attempt_count cannot decrease';
  END IF;

  IF OLD.status IN ('completed', 'dead_letter') AND NEW.status IS DISTINCT FROM OLD.status THEN
    RAISE EXCEPTION 'terminal ingest job status is immutable; create a reprocessing job';
  END IF;

  IF NEW.status IN ('completed', 'dead_letter')
     AND (NEW.worker_lease_token IS NOT NULL OR NEW.worker_lease_expires_at IS NOT NULL) THEN
    RAISE EXCEPTION 'terminal ingest jobs cannot retain a worker lease';
  END IF;

  RETURN NEW;
END
$$;

DROP TRIGGER IF EXISTS records_ingest_jobs_protect_history
  ON forensic.records_ingest_jobs;
CREATE TRIGGER records_ingest_jobs_protect_history
BEFORE UPDATE ON forensic.records_ingest_jobs
FOR EACH ROW EXECUTE FUNCTION forensic.protect_ingest_job_history();

COMMIT;
