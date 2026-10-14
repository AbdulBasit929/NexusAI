-- Read-only compatibility gate for migration 017 (the analyst views and the read-only analyst role). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  missing text;
BEGIN
  IF to_regclass('forensic.comm_event') IS NULL OR to_regclass('forensic.data_session') IS NULL OR to_regclass('forensic.app_usage') IS NULL THEN
    RAISE EXCEPTION 'migration 012 must be applied before migration 017 (a typed table is missing)';
  END IF;
  IF to_regclass('forensic.cell_site') IS NULL THEN
    RAISE EXCEPTION 'migration 013 must be applied before migration 017 (forensic.cell_site is missing)';
  END IF;

  SELECT string_agg(c.tbl || '.' || c.name, ', ')
    INTO missing
  FROM (VALUES
    ('comm_event', 'event_id'), ('comm_event', 'tenant_id'), ('comm_event', 'collection_id'), ('comm_event', 'evidence_id'), ('comm_event', 'source_row'),
    ('comm_event', 'dataset_kind'), ('comm_event', 'event_type'), ('comm_event', 'direction'), ('comm_event', 'a_norm'), ('comm_event', 'a_raw'),
    ('comm_event', 'b_norm'), ('comm_event', 'b_raw'), ('comm_event', 'b_kind'), ('comm_event', 'start_utc'), ('comm_event', 'end_utc'),
    ('comm_event', 'duration_s'), ('comm_event', 'imsi'), ('comm_event', 'imei'), ('comm_event', 'cell_key'), ('comm_event', 'time_basis'),
    ('comm_event', 'source_tz'), ('comm_event', 'quality_flags'),
    ('data_session', 'session_id'), ('data_session', 'msisdn'), ('data_session', 'public_ip'), ('data_session', 'public_port_from'),
    ('data_session', 'public_port_to'), ('data_session', 'nat_start_utc'), ('data_session', 'nat_end_utc'), ('data_session', 'host'), ('data_session', 'bytes_up'), ('data_session', 'quality_flags'),
    ('app_usage', 'usage_id'), ('app_usage', 'msisdn'), ('app_usage', 'app'), ('app_usage', 'province'), ('app_usage', 'quality_flags'),
    ('cell_site', 'cell_key'), ('cell_site', 'site_name'), ('cell_site', 'operator_label'), ('cell_site', 'radius_m'), ('cell_site', 'status'),
    ('cell_site', 'source_rank'), ('cell_site', 'valid_from')
  ) AS c(tbl, name)
  WHERE NOT EXISTS (
    SELECT 1 FROM information_schema.columns k
    WHERE k.table_schema = 'forensic' AND k.table_name = c.tbl AND k.column_name = c.name);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION 'the typed tables lack columns the analyst views read: %', missing;
  END IF;

  IF EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'analyst')
     AND EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
                 WHERE n.nspname = 'analyst' AND c.relkind NOT IN ('v')) THEN
    RAISE EXCEPTION 'schema analyst exists and holds something that is not a view: this migration would not own it';
  END IF;

  IF NOT has_database_privilege(current_user, current_database(), 'CREATE') THEN
    RAISE EXCEPTION 'role % cannot create a schema in this database', current_user;
  END IF;
  IF NOT (SELECT rolcreaterole OR rolsuper FROM pg_roles WHERE rolname = current_user)
     AND NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_analyst_ro') THEN
    RAISE EXCEPTION 'role % cannot create the role forensic_analyst_ro (needs CREATEROLE or superuser)', current_user;
  END IF;

  RAISE NOTICE 'migration 017 preflight passed (schema analyst %, role forensic_analyst_ro %)',
    CASE WHEN EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'analyst') THEN 'present: views will be replaced in place' ELSE 'absent: will be created' END,
    CASE WHEN EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_analyst_ro') THEN 'present' ELSE 'absent: will be created' END;
END
$$;

ROLLBACK;
