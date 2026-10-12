BEGIN TRANSACTION READ ONLY;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM forensic.records_ingest_jobs
    WHERE queue_message_id IS NULL
       OR queue_message_id <> 'ingest-job:' || id::text
       OR queue_publish_attempts < 0
       OR reprocess_generation < 0
       OR attempt_count < 0
       OR attempt_count > max_attempts
       OR (status IN ('completed', 'dead_letter') AND worker_lease_token IS NOT NULL)
       OR ((reprocess_generation = 0) <> (reprocess_of_job_id IS NULL))
  ) THEN
    RAISE EXCEPTION 'Phase 3 queue lifecycle invariant verification failed';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_trigger
    WHERE tgrelid = 'forensic.records_ingest_jobs'::regclass
      AND tgname = 'records_ingest_jobs_protect_history'
      AND NOT tgisinternal
  ) THEN
    RAISE EXCEPTION 'queue lifecycle history trigger is missing';
  END IF;
END
$$;

ROLLBACK;
