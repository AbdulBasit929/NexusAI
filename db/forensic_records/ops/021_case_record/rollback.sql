-- Destructive rollback for 021_case_record.sql: drops the table AND every case record in it (display names and statuses). Export them first if they matter.
-- Nothing else references the table. Run only from a verified backup, with ON_ERROR_STOP enabled and an explicit:
--   SET app.case_record_allow_destructive_rollback = 'true';
BEGIN;
DO $$
DECLARE
  n bigint := 0;
BEGIN
  IF current_setting('app.case_record_allow_destructive_rollback', true) IS DISTINCT FROM 'true' THEN
    RAISE EXCEPTION 'the 021 rollback is destructive; set app.case_record_allow_destructive_rollback=true explicitly' USING ERRCODE = '55000';
  END IF;
  IF to_regclass('forensic.case_record') IS NOT NULL THEN
    EXECUTE 'SELECT count(*) FROM forensic.case_record' INTO n;
    RAISE NOTICE 'dropping forensic.case_record with % rows', n;
  END IF;
END
$$;
DROP TABLE IF EXISTS forensic.case_record;
COMMIT;
