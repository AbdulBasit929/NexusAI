-- Rollback for 017_analyst_views.sql: removes schema analyst (its views only) and the role forensic_analyst_ro. No table, row or policy was changed by the
-- migration, so nothing is lost. Whatever read the views (the Ask engine with FORENSIC_ASK_V2 on) must be switched off first, or its queries fail.
BEGIN;
DROP SCHEMA IF EXISTS analyst CASCADE;
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_analyst_ro') THEN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
      EXECUTE 'REVOKE forensic_analyst_ro FROM forensic_runtime';
    END IF;
    EXECUTE 'DROP ROLE forensic_analyst_ro';
  END IF;
END
$$;
COMMIT;
