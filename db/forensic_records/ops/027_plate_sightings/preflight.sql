-- Read-only compatibility gate for migration 027 (the views v_plate_sightings and v_plate_visits). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  missing text;
BEGIN
  IF to_regclass('forensic.plate_sighting') IS NULL THEN
    RAISE EXCEPTION 'migration 012 must be applied before migration 027 (forensic.plate_sighting is missing)';
  END IF;
  IF to_regnamespace('analyst') IS NULL OR to_regclass('analyst.v_comm_events') IS NULL THEN
    RAISE EXCEPTION 'migration 017 must be applied before migration 027 (schema analyst or analyst.v_comm_events is missing)';
  END IF;
  SELECT string_agg(c.name, ', ') INTO missing
  FROM (VALUES ('tenant_id'), ('collection_id'), ('evidence_id'), ('source_row'), ('plate_raw'), ('plate_norm'), ('observed_utc'), ('time_basis'), ('source_tz'), ('camera_id'),
               ('camera_label'), ('lane'), ('lat'), ('lon'), ('loc_method'), ('confidence'), ('source_kind'), ('review_state'), ('quality_flags'), ('sighting_id')) AS c(name)
  WHERE NOT EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid = 'forensic.plate_sighting'::regclass AND a.attname = c.name AND a.attnum > 0 AND NOT a.attisdropped);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION 'forensic.plate_sighting lacks the columns the plate views read: %', missing;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_analyst_ro') THEN
    RAISE EXCEPTION 'the role forensic_analyst_ro is missing (migration 017)';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
             WHERE n.nspname = 'analyst' AND c.relname IN ('v_plate_sightings', 'v_plate_visits') AND c.relkind <> 'v') THEN
    RAISE EXCEPTION 'a relation named like a new view exists in schema analyst and is not a view';
  END IF;
  RAISE NOTICE 'migration 027 preflight passed';
END
$$;

ROLLBACK;
