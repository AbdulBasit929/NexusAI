-- Destructive rollback for 013_reference_tables.sql: drops the reference tables AND every row loaded into them.
-- Run it only from a verified backup, with ON_ERROR_STOP enabled and an explicit:
--   SET app.reference_tables_allow_destructive_rollback = 'true';
-- Roll back 015 first if it was applied (its earth indexes sit on cell_site and camera_site).
BEGIN;

DO $$
DECLARE
  tbl text;
  n bigint;
BEGIN
  IF current_setting('app.reference_tables_allow_destructive_rollback', true) IS DISTINCT FROM 'true' THEN
    RAISE EXCEPTION 'the 013 rollback is destructive; set app.reference_tables_allow_destructive_rollback=true explicitly'
      USING ERRCODE = '55000';
  END IF;

  FOREACH tbl IN ARRAY ARRAY['cell_site', 'camera_site', 'device_tac', 'ip_range_reference']
  LOOP
    IF to_regclass('forensic.' || tbl) IS NOT NULL THEN
      EXECUTE format('SELECT count(*) FROM forensic.%I', tbl) INTO n;
      RAISE NOTICE 'dropping forensic.% with % rows', tbl, n;
    END IF;
  END LOOP;
END
$$;

DROP TABLE IF EXISTS forensic.ip_range_reference;
DROP TABLE IF EXISTS forensic.device_tac;
DROP TABLE IF EXISTS forensic.camera_site;
DROP TABLE IF EXISTS forensic.cell_site;

DROP FUNCTION IF EXISTS forensic.make_cell_key(text, text, text);
DROP FUNCTION IF EXISTS forensic.norm_cell_part(text);

COMMIT;
