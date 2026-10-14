-- Read-only verification of migration 018. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  problem text;
  v regclass := to_regclass('forensic.records_effective_ipdr');
BEGIN
  IF v IS NULL OR NOT EXISTS (SELECT 1 FROM pg_class c WHERE c.oid = v AND c.relkind = 'v') THEN
    RAISE EXCEPTION 'forensic.records_effective_ipdr is missing or is not a plain view';
  END IF;

  -- The caller's row-level security must apply to the tables beneath the view.
  IF NOT EXISTS (SELECT 1 FROM pg_class c WHERE c.oid = v AND 'security_invoker=true' = ANY (coalesce(c.reloptions, ARRAY[]::text[]))) THEN
    RAISE EXCEPTION 'the view is not security_invoker: tenant isolation would run with the owner''s privileges';
  END IF;

  -- The contract with the SQL builders: the same columns, in the same order, with the same types, as forensic.records.
  SELECT string_agg(format('%s: view has %s, records has %s', coalesce(a.attname, b.attname), coalesce(format_type(a.atttypid, a.atttypmod), 'nothing'),
                           coalesce(format_type(b.atttypid, b.atttypmod), 'nothing')), '; ')
    INTO problem
  FROM (SELECT attnum, attname, atttypid, atttypmod FROM pg_attribute WHERE attrelid = v AND attnum > 0 AND NOT attisdropped) a
  FULL JOIN (SELECT attnum, attname, atttypid, atttypmod FROM pg_attribute WHERE attrelid = 'forensic.records'::regclass AND attnum > 0 AND NOT attisdropped) b
    ON a.attnum = b.attnum
  WHERE a.attname IS DISTINCT FROM b.attname OR a.atttypid IS DISTINCT FROM b.atttypid;
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'the view does not have the columns of forensic.records: %', problem;
  END IF;

  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') AND NOT has_table_privilege('forensic_runtime', v, 'SELECT') THEN
    RAISE EXCEPTION 'forensic_runtime cannot read the view';
  END IF;

  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime')
     AND (has_table_privilege('forensic_runtime', v, 'INSERT') OR has_table_privilege('forensic_runtime', v, 'UPDATE')
          OR has_table_privilege('forensic_runtime', v, 'DELETE')) THEN
    RAISE EXCEPTION 'forensic_runtime must only be able to read the view';
  END IF;

  RAISE NOTICE 'migration_018_verified';
END
$$;

-- It must also run: a plan over the view with the usual scope predicates.
SELECT count(*) AS rows_visible_to_this_session FROM forensic.records_effective_ipdr WHERE tenant_id = 'nobody' AND collection_id = 'nobody';

ROLLBACK;
