-- Read-only compatibility gate for the runtime tenant-policy closure.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  target record;
BEGIN
  IF to_regclass('forensic.evidence_storage_objects') IS NULL
     OR to_regprocedure('forensic.protect_ingest_job_history()') IS NULL THEN
    RAISE EXCEPTION 'migrations 008 and 009 must be applied and verified before migration 010';
  END IF;

  IF to_regclass('forensic.cdr_frequent_contacts') IS NULL
     OR to_regclass('forensic.entity_activity_summary') IS NULL THEN
    RAISE EXCEPTION 'required forensic query views are missing';
  END IF;

  FOR target IN
    SELECT * FROM (VALUES
      ('records_ingest_jobs', 'tenant_isolation_records_ingest_jobs'),
      ('records_ingest_errors', 'tenant_isolation_records_ingest_errors'),
      ('records_audit_log', 'tenant_isolation_records_audit_log')
    ) AS required(table_name, policy_name)
  LOOP
    IF to_regclass('forensic.' || target.table_name) IS NULL THEN
      RAISE EXCEPTION 'required table forensic.% is missing', target.table_name;
    END IF;

    IF NOT EXISTS (
      SELECT 1
      FROM pg_class c
      JOIN pg_namespace n ON n.oid = c.relnamespace
      WHERE n.nspname = 'forensic'
        AND c.relname = target.table_name
        AND c.relrowsecurity
    ) THEN
      RAISE EXCEPTION 'row-level security is not enabled on forensic.%', target.table_name;
    END IF;

    IF EXISTS (
      SELECT 1
      FROM pg_policies
      WHERE schemaname = 'forensic'
        AND tablename = target.table_name
        AND policyname <> target.policy_name
    ) THEN
      RAISE EXCEPTION 'unexpected policy exists on forensic.%: inspect before migration', target.table_name;
    END IF;
  END LOOP;
END
$$;

ROLLBACK;
