-- Read-only compatibility gate for migration 018 (the effective internet-record view). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  missing text;
BEGIN
  IF current_setting('server_version_num')::int < 150000 THEN
    RAISE EXCEPTION 'PostgreSQL 15 or later is required (the view uses security_invoker); this server is %', current_setting('server_version');
  END IF;

  IF to_regclass('forensic.records') IS NULL OR to_regclass('forensic.evidence_items') IS NULL THEN
    RAISE EXCEPTION 'migrations 001 and 007 must be applied before migration 018';
  END IF;

  IF to_regclass('forensic.data_session') IS NULL THEN
    RAISE EXCEPTION 'migration 012 must be applied before migration 018 (forensic.data_session is missing)';
  END IF;

  SELECT string_agg(c.name, ', ')
    INTO missing
  FROM (VALUES ('session_id'), ('tenant_id'), ('collection_id'), ('evidence_id'), ('file_id'), ('batch_id'), ('source_row'), ('record_id'),
               ('row_hash'), ('layout_id'), ('layout_version'), ('msisdn'), ('start_utc'), ('end_utc'), ('ue_ip'), ('ue_port'), ('server_ip'), ('server_port'),
               ('protocol'), ('host'), ('url'), ('bytes_up'), ('bytes_down'), ('cell_key'), ('session_ref'), ('source_tz'), ('time_basis'), ('raw_payload'),
               ('quality_flags'), ('ingested_at')) AS c(name)
  WHERE NOT EXISTS (
    SELECT 1 FROM information_schema.columns k
    WHERE k.table_schema = 'forensic' AND k.table_name = 'data_session' AND k.column_name = c.name);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION 'forensic.data_session lacks the columns the view reads: %', missing;
  END IF;

  IF to_regclass('forensic.records_effective_ipdr') IS NOT NULL
     AND NOT EXISTS (SELECT 1 FROM pg_class c WHERE c.oid = to_regclass('forensic.records_effective_ipdr') AND c.relkind = 'v') THEN
    RAISE EXCEPTION 'forensic.records_effective_ipdr exists and is not a view';
  END IF;

  IF NOT has_schema_privilege(current_user, 'forensic', 'CREATE') THEN
    RAISE EXCEPTION 'role % cannot create objects in schema forensic', current_user;
  END IF;

  RAISE NOTICE 'migration 018 preflight passed (view %)',
    CASE WHEN to_regclass('forensic.records_effective_ipdr') IS NULL THEN 'absent: will be created' ELSE 'present: will be replaced in place' END;
END
$$;

ROLLBACK;
