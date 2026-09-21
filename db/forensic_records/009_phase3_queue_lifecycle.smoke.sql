BEGIN;

DO $$
DECLARE
  parent_job uuid;
  evidence uuid;
  immutable_rejected boolean := false;
BEGIN
  SELECT id, evidence_id INTO parent_job, evidence
  FROM forensic.records_ingest_jobs
  WHERE tenant_id = 'phase3-smoke'
  ORDER BY queued_at
  LIMIT 1;

  IF parent_job IS NULL OR evidence IS NULL THEN
    RAISE EXCEPTION 'Phase 3 control-plane smoke fixture is required';
  END IF;

  INSERT INTO forensic.records_ingest_jobs (
    id, tenant_id, user_id, collection_id, file_id, source_file, spool_path,
    sha256, record_type, status, max_attempts, evidence_id, metadata,
    queue_message_id, queue_publish_next_at, reprocess_of_job_id,
    reprocess_generation, reprocess_request_key
  ) VALUES (
    '90000000-0000-0000-0000-000000000009', 'phase3-smoke', 'smoke-user',
    'phase3-control-plane-smoke', 'phase3-queue-reprocess', 'queue-smoke.csv',
    '/tmp/queue-smoke.csv', repeat('9', 64), 'cdr', 'queued', 2, evidence,
    '{"queue_smoke":true}'::jsonb,
    'ingest-job:90000000-0000-0000-0000-000000000009', now(), parent_job, 1,
    'phase3-queue-smoke-request'
  );

  UPDATE forensic.records_ingest_jobs
  SET status = 'running', attempt_count = 1,
      worker_lease_token = '90000000-0000-0000-0000-000000000001',
      worker_lease_owner = 'phase3-smoke-worker',
      worker_lease_expires_at = now() + interval '5 minutes',
      started_at = now()
  WHERE id = '90000000-0000-0000-0000-000000000009';

  UPDATE forensic.records_ingest_jobs
  SET status = 'failed', next_attempt_at = now() + interval '1 second',
      last_error_class = 'transient', error_message = 'synthetic retry',
      worker_lease_token = NULL, worker_lease_owner = NULL,
      worker_lease_expires_at = NULL
  WHERE id = '90000000-0000-0000-0000-000000000009';

  UPDATE forensic.records_ingest_jobs
  SET status = 'running', attempt_count = 2,
      worker_lease_token = '90000000-0000-0000-0000-000000000002',
      worker_lease_owner = 'phase3-smoke-worker',
      worker_lease_expires_at = now() + interval '5 minutes'
  WHERE id = '90000000-0000-0000-0000-000000000009';

  UPDATE forensic.records_ingest_jobs
  SET status = 'completed', completed_at = now(), acknowledged_at = now(),
      worker_lease_token = NULL, worker_lease_owner = NULL,
      worker_lease_expires_at = NULL
  WHERE id = '90000000-0000-0000-0000-000000000009';

  BEGIN
    UPDATE forensic.records_ingest_jobs
    SET status = 'queued'
    WHERE id = '90000000-0000-0000-0000-000000000009';
  EXCEPTION WHEN OTHERS THEN
    immutable_rejected := true;
  END;
  IF NOT immutable_rejected THEN
    RAISE EXCEPTION 'terminal queue state mutation was not rejected';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM forensic.processing_runs
    WHERE tenant_id = 'phase3-smoke'
      AND idempotency_key = 'ingest-job:90000000-0000-0000-0000-000000000009'
      AND attempt_count = 2
      AND status = 'succeeded'
  ) THEN
    RAISE EXCEPTION 'queue reprocessing job did not synchronize to processing_runs';
  END IF;
END
$$;

ROLLBACK;
