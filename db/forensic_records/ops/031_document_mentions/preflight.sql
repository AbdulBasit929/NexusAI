-- Read-only compatibility gate for migration 031 (forensic.document_mention and the three document views). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  v text;
  has_rows boolean := false;
BEGIN
  IF to_regclass('forensic.evidence_items') IS NULL OR to_regclass('forensic.derived_artifacts') IS NULL THEN
    RAISE EXCEPTION 'migrations 007 and 008 must be applied before migration 031 (forensic.evidence_items or forensic.derived_artifacts is missing)';
  END IF;
  FOREACH v IN ARRAY ARRAY['v_comm_events', 'v_subscriber_registrations', 'v_bank_transactions', 'v_plate_sightings'] LOOP
    IF to_regclass('analyst.' || v) IS NULL THEN
      RAISE EXCEPTION 'analyst.% is missing: migrations 017, 027, 028 and 030 must be applied before migration 031 (the link view reads it)', v;
    END IF;
  END LOOP;
  IF to_regclass('forensic.document_mention') IS NOT NULL THEN
    EXECUTE 'SELECT EXISTS (SELECT 1 FROM forensic.document_mention)' INTO has_rows;
    IF has_rows THEN
      RAISE NOTICE 'forensic.document_mention exists and holds rows: the migration is idempotent and leaves them';
    END IF;
  END IF;
  RAISE NOTICE 'migration 031 preflight passed';
END
$$;

ROLLBACK;
