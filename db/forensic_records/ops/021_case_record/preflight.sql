-- Read-only compatibility gate for migration 021 (the case record). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;
DO $$
BEGIN
  IF to_regnamespace('forensic') IS NULL THEN
    RAISE EXCEPTION 'schema forensic is missing';
  END IF;
  IF to_regclass('forensic.evidence_items') IS NULL THEN
    RAISE EXCEPTION 'migration 007 (the evidence registry) must be applied first';
  END IF;
  IF to_regclass('forensic.case_record') IS NOT NULL
     AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'forensic' AND table_name = 'case_record' AND column_name = 'display_name') THEN
    RAISE EXCEPTION 'forensic.case_record exists but is not the table migration 021 describes';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    RAISE NOTICE 'the role forensic_runtime does not exist: its grants will be skipped (a fresh database is given them by runtime_role.grants.sql)';
  END IF;
  RAISE NOTICE 'migration 021 preflight passed';
END
$$;
ROLLBACK;
