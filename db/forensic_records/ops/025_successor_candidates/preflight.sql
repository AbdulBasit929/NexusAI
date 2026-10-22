-- Read-only compatibility gate for migration 025 (the views v_silent_numbers and v_successor_candidates). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  missing text;
BEGIN
  IF to_regclass('analyst.v_comm_events') IS NULL THEN
    RAISE EXCEPTION 'migration 017 must be applied before migration 025 (analyst.v_comm_events is missing)';
  END IF;
  SELECT string_agg(c.name, ', ') INTO missing
  FROM (VALUES ('src'), ('dst'), ('dst_is_person_like'), ('cell_key'), ('imei'), ('start_utc')) AS c(name)
  WHERE NOT EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid = 'analyst.v_comm_events'::regclass AND a.attname = c.name AND a.attnum > 0 AND NOT a.attisdropped);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION 'analyst.v_comm_events lacks the columns the successor views read: %', missing;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_analyst_ro') THEN
    RAISE EXCEPTION 'the role forensic_analyst_ro is missing (migration 017)';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
             WHERE n.nspname = 'analyst' AND c.relname IN ('v_silent_numbers', 'v_successor_candidates') AND c.relkind <> 'v') THEN
    RAISE EXCEPTION 'a relation named like a new view exists in schema analyst and is not a view';
  END IF;
  RAISE NOTICE 'migration 025 preflight passed';
END
$$;

ROLLBACK;
