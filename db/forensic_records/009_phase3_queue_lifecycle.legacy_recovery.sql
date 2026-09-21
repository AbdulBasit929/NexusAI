-- One-time retained-registry recovery for the seven historical failed jobs
-- diagnosed on 2026-07-29. This is not a general migration or retry mechanism.
-- It preserves the original jobs as dead-letter history and fixes their exact
-- evidence/job pointers before migration 009.
--
-- Run only from a verified post-008 backup and with the explicit session gate.

BEGIN;

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

DO $$
BEGIN
  IF current_setting('app.phase3_allow_legacy_queue_recovery', true) <> 'true' THEN
    RAISE EXCEPTION
      'legacy queue recovery is gated; set app.phase3_allow_legacy_queue_recovery=true explicitly';
  END IF;

  IF to_regclass('forensic.processing_runs') IS NULL
     OR to_regclass('forensic.processing_events') IS NULL THEN
    RAISE EXCEPTION 'migration 008 must be applied and verified first';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'forensic'
      AND table_name = 'records_ingest_jobs'
      AND column_name IN (
        'queue_message_id', 'worker_lease_token', 'reprocess_generation'
      )
  ) THEN
    RAISE EXCEPTION 'legacy recovery must run before migration 009';
  END IF;

  PERFORM pg_advisory_xact_lock(
    hashtextextended('phase3-legacy-queue-recovery-v1', 0)
  );

  PERFORM job.id
  FROM forensic.records_ingest_jobs job
  WHERE job.status = 'failed'
    AND encode(digest(job.error_message, 'sha256'), 'hex') IN (
      '7aa916a87064b2a3cf26b2c8efb75c31948a6768235933c508db03fd1a5d4730',
      '73a4d88751c76153db44d15ce025e9a44359851fe20fbf0cf1cee6e931a88cc2'
    )
  ORDER BY job.id
  FOR UPDATE;

  PERFORM evidence.evidence_id
  FROM forensic.evidence_items evidence
  JOIN forensic.records_ingest_jobs job
    ON job.tenant_id = evidence.tenant_id
   AND job.collection_id = evidence.collection_id
   AND job.evidence_id = evidence.evidence_id
  WHERE job.status = 'failed'
    AND encode(digest(job.error_message, 'sha256'), 'hex') IN (
      '7aa916a87064b2a3cf26b2c8efb75c31948a6768235933c508db03fd1a5d4730',
      '73a4d88751c76153db44d15ce025e9a44359851fe20fbf0cf1cee6e931a88cc2'
    )
  ORDER BY evidence.evidence_id
  FOR UPDATE;
END
$$;

CREATE TEMP TABLE phase3_legacy_recovery_targets (
  job_id uuid PRIMARY KEY,
  evidence_id uuid UNIQUE NOT NULL,
  tenant_id text NOT NULL,
  collection_id text NOT NULL,
  file_id text NOT NULL,
  user_id text NOT NULL,
  record_type text NOT NULL,
  prior_evidence_status text NOT NULL,
  diagnostic_sha256 text NOT NULL,
  attempt_count integer NOT NULL,
  max_attempts integer NOT NULL,
  completed_at timestamptz NOT NULL
) ON COMMIT DROP;

INSERT INTO phase3_legacy_recovery_targets (
  job_id, evidence_id, tenant_id, collection_id, file_id, user_id,
  record_type, prior_evidence_status, diagnostic_sha256,
  attempt_count, max_attempts, completed_at
)
SELECT
  job.id, job.evidence_id, job.tenant_id, job.collection_id, job.file_id,
  job.user_id, job.record_type::text, evidence.processing_status,
  encode(digest(job.error_message, 'sha256'), 'hex'),
  job.attempt_count, job.max_attempts, job.completed_at
FROM forensic.records_ingest_jobs job
JOIN forensic.evidence_items evidence
  ON evidence.tenant_id = job.tenant_id
 AND evidence.collection_id = job.collection_id
 AND evidence.evidence_id = job.evidence_id
WHERE job.status = 'failed'
  AND encode(digest(job.error_message, 'sha256'), 'hex') IN (
    '7aa916a87064b2a3cf26b2c8efb75c31948a6768235933c508db03fd1a5d4730',
    '73a4d88751c76153db44d15ce025e9a44359851fe20fbf0cf1cee6e931a88cc2'
  );

DO $$
DECLARE
  actual bigint;
BEGIN
  SELECT count(*) INTO actual FROM phase3_legacy_recovery_targets;
  IF actual <> 7 THEN
    RAISE EXCEPTION 'expected exactly 7 recovery targets, found %', actual;
  END IF;

  SELECT count(*) INTO actual
  FROM phase3_legacy_recovery_targets
  WHERE diagnostic_sha256 =
    '7aa916a87064b2a3cf26b2c8efb75c31948a6768235933c508db03fd1a5d4730';
  IF actual <> 6 THEN
    RAISE EXCEPTION 'expected 6 type-signature jobs, found %', actual;
  END IF;

  SELECT count(*) INTO actual
  FROM phase3_legacy_recovery_targets
  WHERE diagnostic_sha256 =
    '73a4d88751c76153db44d15ce025e9a44359851fe20fbf0cf1cee6e931a88cc2';
  IF actual <> 1 THEN
    RAISE EXCEPTION 'expected 1 missing-adapter-signature job, found %', actual;
  END IF;

  IF EXISTS (
    SELECT 1 FROM phase3_legacy_recovery_targets
    WHERE attempt_count <> 1 OR max_attempts <> 5
  ) THEN
    RAISE EXCEPTION 'legacy attempt invariants drifted';
  END IF;

  SELECT count(*) INTO actual
  FROM phase3_legacy_recovery_targets
  WHERE record_type = 'cdr';
  IF actual <> 1 THEN RAISE EXCEPTION 'expected 1 CDR target, found %', actual; END IF;

  SELECT count(*) INTO actual
  FROM phase3_legacy_recovery_targets
  WHERE record_type = 'anpr';
  IF actual <> 2 THEN RAISE EXCEPTION 'expected 2 ANPR targets, found %', actual; END IF;

  SELECT count(*) INTO actual
  FROM phase3_legacy_recovery_targets
  WHERE record_type = 'ipdr';
  IF actual <> 2 THEN RAISE EXCEPTION 'expected 2 IPDR targets, found %', actual; END IF;

  SELECT count(*) INTO actual
  FROM phase3_legacy_recovery_targets
  WHERE record_type = 'access_log';
  IF actual <> 2 THEN RAISE EXCEPTION 'expected 2 access-log targets, found %', actual; END IF;

  SELECT count(*) INTO actual
  FROM phase3_legacy_recovery_targets
  WHERE prior_evidence_status = 'queued';
  IF actual <> 6 THEN
    RAISE EXCEPTION 'expected 6 stale queued evidence states, found %', actual;
  END IF;

  SELECT count(*) INTO actual
  FROM phase3_legacy_recovery_targets
  WHERE prior_evidence_status = 'failed';
  IF actual <> 1 THEN
    RAISE EXCEPTION 'expected 1 already-failed evidence state, found %', actual;
  END IF;

  SELECT count(*) INTO actual FROM forensic.records_ingest_jobs;
  IF actual <> 41 THEN RAISE EXCEPTION 'expected 41 total jobs, found %', actual; END IF;

  SELECT count(*) INTO actual
  FROM forensic.records_ingest_jobs WHERE status = 'completed';
  IF actual <> 34 THEN RAISE EXCEPTION 'expected 34 completed jobs, found %', actual; END IF;

  SELECT count(*) INTO actual
  FROM forensic.records_ingest_jobs WHERE status = 'failed';
  IF actual <> 7 THEN RAISE EXCEPTION 'expected 7 failed jobs, found %', actual; END IF;

  IF EXISTS (
    SELECT 1
    FROM forensic.records_ingest_jobs job
    JOIN phase3_legacy_recovery_targets target
      ON target.evidence_id = job.evidence_id
    WHERE job.id <> target.job_id
  ) THEN
    RAISE EXCEPTION 'a recovery target has an unexpected sibling ingest job';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM forensic.processing_runs run
    JOIN phase3_legacy_recovery_targets target ON target.job_id = run.run_id
    WHERE run.status <> 'failed'
  ) OR (
    SELECT count(*)
    FROM forensic.processing_runs run
    JOIN phase3_legacy_recovery_targets target ON target.job_id = run.run_id
  ) <> 7 THEN
    RAISE EXCEPTION 'target processing-run state drifted';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM forensic.records_ingest_jobs job
    JOIN phase3_legacy_recovery_targets target ON target.job_id = job.id
    WHERE job.total_rows <> 0 OR job.accepted_rows <> 0
       OR job.duplicate_rows <> 0 OR job.rejected_rows <> 0
  ) THEN
    RAISE EXCEPTION 'a recovery target has non-zero row accounting';
  END IF;

  IF EXISTS (
    SELECT 1 FROM forensic.cdr_records row
    JOIN phase3_legacy_recovery_targets target ON target.job_id = row.batch_id
  ) OR EXISTS (
    SELECT 1 FROM forensic.generic_records row
    JOIN phase3_legacy_recovery_targets target ON target.job_id = row.batch_id
  ) OR EXISTS (
    SELECT 1 FROM forensic.records row
    JOIN phase3_legacy_recovery_targets target ON target.job_id = row.batch_id
  ) OR EXISTS (
    SELECT 1 FROM forensic.kb_active_metadata metadata
    JOIN phase3_legacy_recovery_targets target ON target.job_id = metadata.batch_id
  ) OR EXISTS (
    SELECT 1 FROM forensic.records_ingest_errors error
    JOIN phase3_legacy_recovery_targets target ON target.job_id = error.job_id
  ) OR EXISTS (
    SELECT 1 FROM forensic.records_audit_log audit
    JOIN phase3_legacy_recovery_targets target ON target.job_id = audit.batch_id
  ) THEN
    RAISE EXCEPTION 'a recovery target has unexpected materialized or audit rows';
  END IF;

  IF EXISTS (
    SELECT 1 FROM forensic.kb_collection_assets asset
    JOIN phase3_legacy_recovery_targets target
      ON target.evidence_id = asset.evidence_id
  ) THEN
    RAISE EXCEPTION 'a recovery target has an unexpected KB asset';
  END IF;

  SELECT count(*) INTO actual
  FROM forensic.evidence_storage_objects storage
  JOIN forensic.evidence_versions version
    ON version.storage_object_id = storage.storage_object_id
  JOIN forensic.evidence_items evidence
    ON evidence.current_version_id = version.version_id
  JOIN phase3_legacy_recovery_targets target
    ON target.evidence_id = evidence.evidence_id
  WHERE storage.storage_backend = 'legacy_spool'
    AND storage.immutability_state = 'unverified'
    AND NOT storage.write_once;
  IF actual <> 7 THEN
    RAISE EXCEPTION 'expected 7 legacy unverified storage objects, found %', actual;
  END IF;
END
$$;

CREATE TEMP TABLE phase3_legacy_recovery_baseline ON COMMIT DROP AS
SELECT
  (SELECT count(*) FROM forensic.evidence_items) AS evidence_items,
  (SELECT count(*) FROM forensic.evidence_versions) AS evidence_versions,
  (SELECT count(*) FROM forensic.evidence_source_links) AS source_links,
  (SELECT count(*) FROM forensic.records_ingest_jobs) AS ingest_jobs,
  (SELECT count(*) FROM forensic.processing_runs) AS processing_runs,
  (SELECT count(*) FROM forensic.processing_events) AS processing_events,
  (SELECT count(*) FROM forensic.evidence_custody_events) AS custody_events,
  (SELECT count(*) FROM forensic.records_audit_log) AS audit_rows;

INSERT INTO forensic.records_audit_log (
  tenant_id, user_id, action, collection_id, file_id, batch_id, details
)
SELECT
  target.tenant_id,
  target.user_id,
  'legacy_job_terminalized_before_queue_migration',
  target.collection_id,
  target.file_id,
  target.job_id,
  jsonb_build_object(
    'recovery_version', 'phase3-legacy-failed-to-dead-letter-v1',
    'approval_reference', 'user-approved-phase3-database-completion-2026-07-29',
    'prior_job_status', 'failed',
    'new_job_status', 'dead_letter',
    'prior_evidence_status', target.prior_evidence_status,
    'diagnostic_sha256', target.diagnostic_sha256,
    'source_bytes_reprocessed', false,
    'reason', 'Preserve historical failure truthfully before migration 009'
  )
FROM phase3_legacy_recovery_targets target;

UPDATE forensic.records_ingest_jobs job
SET status = 'dead_letter'
FROM phase3_legacy_recovery_targets target
WHERE job.id = target.job_id
  AND job.status = 'failed';

UPDATE forensic.evidence_items evidence
SET records_batch_id = target.job_id,
    processing_status = CASE
      WHEN evidence.processing_status = 'queued' THEN 'failed'
      ELSE evidence.processing_status
    END,
    updated_at = now()
FROM phase3_legacy_recovery_targets target
WHERE evidence.tenant_id = target.tenant_id
  AND evidence.collection_id = target.collection_id
  AND evidence.evidence_id = target.evidence_id
  AND (
    evidence.records_batch_id IS DISTINCT FROM target.job_id
    OR evidence.processing_status = 'queued'
  );

DO $$
DECLARE
  baseline phase3_legacy_recovery_baseline%ROWTYPE;
  actual bigint;
BEGIN
  SELECT * INTO STRICT baseline FROM phase3_legacy_recovery_baseline;

  SELECT count(*) INTO actual FROM forensic.records_ingest_jobs;
  IF actual <> baseline.ingest_jobs THEN RAISE EXCEPTION 'job count drifted'; END IF;

  SELECT count(*) INTO actual
  FROM forensic.records_ingest_jobs WHERE status = 'completed';
  IF actual <> 34 THEN RAISE EXCEPTION 'completed-job count drifted: %', actual; END IF;

  SELECT count(*) INTO actual
  FROM forensic.records_ingest_jobs WHERE status = 'dead_letter';
  IF actual <> 7 THEN RAISE EXCEPTION 'expected 7 dead-letter jobs, found %', actual; END IF;

  SELECT count(*) INTO actual
  FROM forensic.records_ingest_jobs
  WHERE status IN ('queued', 'running', 'failed');
  IF actual <> 0 THEN RAISE EXCEPTION '% non-terminal jobs remain', actual; END IF;

  IF EXISTS (
    SELECT 1
    FROM forensic.records_ingest_jobs job
    JOIN phase3_legacy_recovery_targets target ON target.job_id = job.id
    WHERE job.status <> 'dead_letter'
      OR job.attempt_count <> target.attempt_count
      OR job.max_attempts <> target.max_attempts
      OR job.completed_at IS DISTINCT FROM target.completed_at
      OR encode(digest(job.error_message, 'sha256'), 'hex')
         <> target.diagnostic_sha256
  ) THEN
    RAISE EXCEPTION 'job history changed beyond approved terminal status';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM forensic.evidence_items evidence
    JOIN phase3_legacy_recovery_targets target
      ON target.tenant_id = evidence.tenant_id
     AND target.collection_id = evidence.collection_id
     AND target.evidence_id = evidence.evidence_id
    WHERE evidence.processing_status <> 'failed'
       OR evidence.records_batch_id <> target.job_id
  ) THEN
    RAISE EXCEPTION 'evidence status or job pointer recovery failed';
  END IF;

  SELECT count(*) INTO actual
  FROM forensic.processing_runs run
  JOIN phase3_legacy_recovery_targets target ON target.job_id = run.run_id
  WHERE run.status = 'dead_letter';
  IF actual <> 7 THEN RAISE EXCEPTION 'expected 7 dead-letter runs, found %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.processing_events;
  IF actual <> baseline.processing_events + 7 THEN
    RAISE EXCEPTION 'expected 7 appended processing events, found delta %',
      actual - baseline.processing_events;
  END IF;

  SELECT count(*) INTO actual FROM forensic.evidence_custody_events;
  IF actual <> baseline.custody_events + 6 THEN
    RAISE EXCEPTION 'expected 6 appended custody events, found delta %',
      actual - baseline.custody_events;
  END IF;

  SELECT count(*) INTO actual FROM forensic.records_audit_log;
  IF actual <> baseline.audit_rows + 7 THEN
    RAISE EXCEPTION 'expected 7 appended audit rows, found delta %',
      actual - baseline.audit_rows;
  END IF;

  SELECT count(*) INTO actual FROM forensic.evidence_items;
  IF actual <> baseline.evidence_items THEN RAISE EXCEPTION 'evidence count drifted'; END IF;
  SELECT count(*) INTO actual FROM forensic.evidence_versions;
  IF actual <> baseline.evidence_versions THEN RAISE EXCEPTION 'version count drifted'; END IF;
  SELECT count(*) INTO actual FROM forensic.evidence_source_links;
  IF actual <> baseline.source_links THEN RAISE EXCEPTION 'source-link count drifted'; END IF;
  SELECT count(*) INTO actual FROM forensic.processing_runs;
  IF actual <> baseline.processing_runs THEN RAISE EXCEPTION 'processing-run count drifted'; END IF;
END
$$;

SELECT
  (SELECT count(*) FROM forensic.records_ingest_jobs WHERE status = 'completed')
    AS completed_jobs,
  (SELECT count(*) FROM forensic.records_ingest_jobs WHERE status = 'dead_letter')
    AS dead_letter_jobs,
  (SELECT count(*) FROM forensic.records_ingest_jobs
    WHERE status IN ('queued', 'running', 'failed')) AS blocking_jobs,
  (SELECT count(*) FROM forensic.processing_runs WHERE status = 'dead_letter')
    AS dead_letter_runs,
  (SELECT count(*) FROM forensic.records_audit_log
    WHERE action = 'legacy_job_terminalized_before_queue_migration')
    AS recovery_audit_rows,
  'phase3_legacy_queue_recovery_passed' AS result;

COMMIT;
