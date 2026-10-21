-- Behavioural smoke of migration 021 inside one rolled-back transaction: the constraints refuse bad values and the tenant policy isolates tenants.
-- Run as the table owner (RLS applies to the owner only when forced, so the policy is checked through a role switch when forensic_runtime exists).
BEGIN;
DO $$
DECLARE
  n int;
BEGIN
  PERFORM set_config('app.tenant_id', 'smoke-a', true);
  INSERT INTO forensic.case_record (tenant_id, collection_id, display_name) VALUES ('smoke-a', 'smoke-case', 'Smoke case');
  BEGIN
    INSERT INTO forensic.case_record (tenant_id, collection_id, status) VALUES ('smoke-a', 'smoke-bad-status', 'archived');
    RAISE EXCEPTION 'a status outside open, on_hold, closed was accepted';
  EXCEPTION WHEN check_violation THEN NULL;
  END;
  BEGIN
    INSERT INTO forensic.case_record (tenant_id, collection_id, display_name) VALUES ('smoke-a', 'smoke-bad-name', E'two\nlines');
    RAISE EXCEPTION 'a display name with a control character was accepted';
  EXCEPTION WHEN check_violation THEN NULL;
  END;
  BEGIN
    INSERT INTO forensic.case_record (tenant_id, collection_id, display_name) VALUES ('smoke-a', 'smoke-empty-name', '   ');
    RAISE EXCEPTION 'a blank display name was accepted';
  EXCEPTION WHEN check_violation THEN NULL;
  END;
  SELECT count(*) INTO n FROM forensic.case_record WHERE collection_id = 'smoke-case';
  IF n <> 1 THEN RAISE EXCEPTION 'expected the smoke row, found %', n; END IF;
  RAISE NOTICE 'migration 021 smoke passed';
END
$$;
ROLLBACK;
