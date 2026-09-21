-- Read-only compatibility gate for Phase 3 durable queue lifecycle.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  active_jobs bigint;
BEGIN
  IF to_regclass('forensic.processing_runs') IS NULL
     OR to_regclass('forensic.processing_events') IS NULL THEN
    RAISE EXCEPTION 'migration 008 must be applied and verified before migration 009';
  END IF;

  SELECT count(*) INTO active_jobs
  FROM forensic.records_ingest_jobs
  WHERE status IN ('queued', 'running', 'failed');
  IF active_jobs <> 0 THEN
    RAISE EXCEPTION
      'queue migration requires a drained legacy worker: % non-terminal jobs require explicit recovery',
      active_jobs;
  END IF;

  IF EXISTS (
    SELECT 1 FROM forensic.records_ingest_jobs
    WHERE attempt_count < 0 OR max_attempts <= 0 OR attempt_count > max_attempts
  ) THEN
    RAISE EXCEPTION 'invalid legacy attempt counters block queue migration';
  END IF;
END
$$;

ROLLBACK;
