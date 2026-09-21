\if :{?phase3_allow_destructive_rollback}
\else
\echo 'Refusing Phase 3 queue rollback. Set phase3_allow_destructive_rollback=true after verifying a backup.'
\quit 3
\endif

\if :phase3_allow_destructive_rollback
\else
\echo 'Refusing Phase 3 queue rollback: phase3_allow_destructive_rollback is not true.'
\quit 3
\endif

BEGIN;

DROP TRIGGER IF EXISTS records_ingest_jobs_protect_history
  ON forensic.records_ingest_jobs;
DROP FUNCTION IF EXISTS forensic.protect_ingest_job_history();

ALTER TABLE forensic.records_ingest_jobs
  DROP CONSTRAINT IF EXISTS records_ingest_jobs_reprocess_parent_fk,
  DROP CONSTRAINT IF EXISTS records_ingest_jobs_queue_counters_chk,
  DROP CONSTRAINT IF EXISTS records_ingest_jobs_reprocess_identity_chk;

DROP INDEX IF EXISTS forensic.records_ingest_jobs_publish_due_idx;
DROP INDEX IF EXISTS forensic.records_ingest_jobs_worker_due_idx;
DROP INDEX IF EXISTS forensic.records_ingest_jobs_worker_lease_idx;
DROP INDEX IF EXISTS forensic.records_ingest_jobs_reprocess_request_uidx;

ALTER TABLE forensic.records_ingest_jobs
  DROP COLUMN IF EXISTS queue_message_id,
  DROP COLUMN IF EXISTS queue_published_at,
  DROP COLUMN IF EXISTS queue_publish_attempts,
  DROP COLUMN IF EXISTS queue_publish_next_at,
  DROP COLUMN IF EXISTS queue_publish_error,
  DROP COLUMN IF EXISTS queue_publish_lease_token,
  DROP COLUMN IF EXISTS queue_publish_lease_expires_at,
  DROP COLUMN IF EXISTS worker_lease_token,
  DROP COLUMN IF EXISTS worker_lease_owner,
  DROP COLUMN IF EXISTS worker_lease_expires_at,
  DROP COLUMN IF EXISTS next_attempt_at,
  DROP COLUMN IF EXISTS last_error_class,
  DROP COLUMN IF EXISTS dead_lettered_at,
  DROP COLUMN IF EXISTS acknowledged_at,
  DROP COLUMN IF EXISTS reprocess_of_job_id,
  DROP COLUMN IF EXISTS reprocess_generation,
  DROP COLUMN IF EXISTS reprocess_request_key;

COMMIT;
