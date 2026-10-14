-- Destructive rollback for 012_typed_families.sql: drops the typed tables AND every row loaded into them.
-- Run it only from a verified backup, with ON_ERROR_STOP enabled and an explicit:
--   SET app.typed_families_allow_destructive_rollback = 'true';
-- Nothing outside migration 012 is touched: the legacy tables (forensic.records, forensic.cdr_records, ...) and the
-- evidence registry are not modified by 012 and are not modified here. The pg_trgm extension is left in place.
BEGIN;

DO $$
DECLARE
  tbl text;
  n bigint;
BEGIN
  IF current_setting('app.typed_families_allow_destructive_rollback', true) IS DISTINCT FROM 'true' THEN
    RAISE EXCEPTION 'the 012 rollback is destructive; set app.typed_families_allow_destructive_rollback=true explicitly'
      USING ERRCODE = '55000';
  END IF;

  -- The newest migration first: 016 adds a view over comm_event.
  IF to_regclass('forensic.records_effective_cdr') IS NOT NULL THEN
    RAISE EXCEPTION 'roll back migration 016 before 012 (the effective call-record view reads comm_event)'
      USING ERRCODE = '55000';
  END IF;

  -- Later migrations (013 to 015) must be rolled back first if they were applied: they keep no foreign key to these
  -- tables today, but the relationship layer's evidence rows point at them by name.
  IF to_regclass('forensic.relationship_evidence') IS NOT NULL THEN
    RAISE EXCEPTION 'roll back migrations 015, 014 and 013 before 012 (relationship_evidence points at the typed tables)'
      USING ERRCODE = '55000';
  END IF;

  FOREACH tbl IN ARRAY ARRAY['comm_event', 'data_session', 'app_usage', 'plate_sighting', 'subscriber_registration', 'subscriber_identity']
  LOOP
    IF to_regclass('forensic.' || tbl) IS NOT NULL THEN
      EXECUTE format('SELECT count(*) FROM forensic.%I', tbl) INTO n;
      RAISE NOTICE 'dropping forensic.% with % rows', tbl, n;
    END IF;
  END LOOP;
END
$$;

DROP TABLE IF EXISTS forensic.subscriber_registration;
DROP TABLE IF EXISTS forensic.subscriber_identity;
DROP TABLE IF EXISTS forensic.plate_sighting;
DROP TABLE IF EXISTS forensic.app_usage;
DROP TABLE IF EXISTS forensic.data_session;
DROP TABLE IF EXISTS forensic.comm_event;
DROP TABLE IF EXISTS forensic.layout_versions;

DROP FUNCTION IF EXISTS forensic.normalize_plate(text);

COMMIT;
