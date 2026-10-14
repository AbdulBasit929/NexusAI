-- Read-only compatibility gate for migration 016 (the effective call-record view). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  missing text;
BEGIN
  IF current_setting('server_version_num')::int < 150000 THEN
    RAISE EXCEPTION 'PostgreSQL 15 or later is required (the view uses security_invoker); this server is %', current_setting('server_version');
  END IF;

  IF to_regclass('forensic.records') IS NULL OR to_regclass('forensic.evidence_items') IS NULL THEN
    RAISE EXCEPTION 'migrations 001 and 007 must be applied before migration 016';
  END IF;

  IF to_regclass('forensic.comm_event') IS NULL THEN
    RAISE EXCEPTION 'migration 012 must be applied before migration 016 (forensic.comm_event is missing)';
  END IF;

  SELECT string_agg(c.name, ', ')
    INTO missing
  FROM (VALUES ('event_id'), ('tenant_id'), ('collection_id'), ('evidence_id'), ('file_id'), ('batch_id'), ('source_row'), ('record_id'),
               ('row_hash'), ('layout_id'), ('layout_version'), ('dataset_kind'), ('event_type'), ('direction'), ('a_norm'), ('b_norm'),
               ('start_utc'), ('end_utc'), ('source_tz'), ('time_basis'), ('imsi'), ('imei'), ('lat'), ('lon'), ('raw_payload'),
               ('quality_flags'), ('ingested_at')) AS c(name)
  WHERE NOT EXISTS (
    SELECT 1 FROM information_schema.columns k
    WHERE k.table_schema = 'forensic' AND k.table_name = 'comm_event' AND k.column_name = c.name);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION 'forensic.comm_event lacks the columns the view reads: %', missing;
  END IF;

  IF to_regclass('forensic.records_effective_cdr') IS NOT NULL
     AND NOT EXISTS (SELECT 1 FROM pg_class c WHERE c.oid = to_regclass('forensic.records_effective_cdr') AND c.relkind = 'v') THEN
    RAISE EXCEPTION 'forensic.records_effective_cdr exists and is not a view';
  END IF;

  IF NOT has_schema_privilege(current_user, 'forensic', 'CREATE') THEN
    RAISE EXCEPTION 'role % cannot create objects in schema forensic', current_user;
  END IF;

  RAISE NOTICE 'migration 016 preflight passed (view %)',
    CASE WHEN to_regclass('forensic.records_effective_cdr') IS NULL THEN 'absent: will be created' ELSE 'present: will be replaced in place' END;
END
$$;

ROLLBACK;
