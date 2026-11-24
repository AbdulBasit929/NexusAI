-- Read-only verification of migration 039. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  problem text;
  f text;
  t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['image_known_set', 'image_known_hash', 'image_known_event']
  LOOP
    IF to_regclass('forensic.' || t) IS NULL THEN
      RAISE EXCEPTION 'forensic.% is missing', t;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_class c WHERE c.oid = ('forensic.' || t)::regclass AND c.relrowsecurity) THEN
      RAISE EXCEPTION 'row-level security is not enabled on forensic.%', t;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_policies p WHERE p.schemaname = 'forensic' AND p.tablename = t AND p.policyname = 'tenant_isolation_' || t
                   AND p.cmd = 'ALL' AND p.qual IS NOT NULL AND p.with_check IS NOT NULL) THEN
      RAISE EXCEPTION 'tenant policy missing on forensic.%', t;
    END IF;
  END LOOP;

  -- hash-only: no column of these tables can hold picture bytes
  SELECT string_agg(k.table_name || '.' || k.column_name, ', ') INTO problem
  FROM information_schema.columns k
  WHERE k.table_schema = 'forensic' AND k.table_name IN ('image_known_set', 'image_known_hash', 'image_known_event') AND k.data_type = 'bytea';
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'a known-set table has a bytea column (the sets are hash-only): %', problem;
  END IF;

  SELECT string_agg(c.conname, ', ') INTO problem
  FROM (VALUES ('image_known_set', 'image_known_set_kind_chk'), ('image_known_set', 'image_known_set_name_chk'), ('image_known_set', 'image_known_set_retired_chk'),
               ('image_known_hash', 'image_known_hash_set_fk'), ('image_known_hash', 'image_known_hash_any_chk'), ('image_known_hash', 'image_known_hash_sha_chk'),
               ('image_known_hash', 'image_known_hash_source_chk'), ('image_known_hash', 'image_known_hash_label_chk'), ('image_known_hash', 'image_known_hash_ref_chk'),
               ('image_known_event', 'image_known_event_kind_chk'), ('image_known_event', 'image_known_event_actor_chk'), ('image_known_event', 'image_known_event_set_fk')) AS c(tbl, conname)
  WHERE NOT EXISTS (SELECT 1 FROM pg_constraint k WHERE k.conrelid = ('forensic.' || c.tbl)::regclass AND k.conname = c.conname AND k.convalidated);
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'missing or unvalidated constraints: %', problem;
  END IF;

  FOREACH f IN ARRAY ARRAY['image_known_matches(text,integer,text,uuid[])', 'image_known_state(text,text)']
  LOOP
    IF to_regprocedure('forensic.' || f) IS NULL THEN
      RAISE EXCEPTION 'function forensic.% is missing', f;
    END IF;
    -- SECURITY INVOKER keeps row-level security in force inside the function.
    IF EXISTS (SELECT 1 FROM pg_proc p WHERE p.oid = to_regprocedure('forensic.' || f) AND p.prosecdef) THEN
      RAISE EXCEPTION 'function forensic.% is SECURITY DEFINER; it must be SECURITY INVOKER', f;
    END IF;
  END LOOP;

  IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = 'forensic' AND indexname = 'image_known_set_active_name_idx')
     OR NOT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = 'forensic' AND indexname = 'image_known_hash_unique_idx') THEN
    RAISE EXCEPTION 'a unique index of the known sets is missing';
  END IF;

  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    IF NOT has_table_privilege('forensic_runtime', 'forensic.image_known_set', 'SELECT')
       OR NOT has_table_privilege('forensic_runtime', 'forensic.image_known_set', 'INSERT')
       OR has_table_privilege('forensic_runtime', 'forensic.image_known_set', 'DELETE')
       OR NOT has_column_privilege('forensic_runtime', 'forensic.image_known_set', 'retired_at', 'UPDATE')
       OR NOT has_column_privilege('forensic_runtime', 'forensic.image_known_set', 'retired_by', 'UPDATE')
       OR has_column_privilege('forensic_runtime', 'forensic.image_known_set', 'name', 'UPDATE')
       OR has_column_privilege('forensic_runtime', 'forensic.image_known_set', 'kind', 'UPDATE') THEN
      RAISE EXCEPTION 'runtime role privileges are wrong on image_known_set (SELECT, INSERT, and UPDATE of the retirement columns only)';
    END IF;
    IF NOT has_table_privilege('forensic_runtime', 'forensic.image_known_hash', 'SELECT')
       OR NOT has_table_privilege('forensic_runtime', 'forensic.image_known_hash', 'INSERT')
       OR has_table_privilege('forensic_runtime', 'forensic.image_known_hash', 'UPDATE')
       OR has_table_privilege('forensic_runtime', 'forensic.image_known_hash', 'DELETE')
       OR NOT has_table_privilege('forensic_runtime', 'forensic.image_known_event', 'SELECT')
       OR NOT has_table_privilege('forensic_runtime', 'forensic.image_known_event', 'INSERT')
       OR has_table_privilege('forensic_runtime', 'forensic.image_known_event', 'UPDATE')
       OR has_table_privilege('forensic_runtime', 'forensic.image_known_event', 'DELETE') THEN
      RAISE EXCEPTION 'runtime role privileges are wrong on image_known_hash or image_known_event (SELECT and INSERT only)';
    END IF;
  END IF;
END
$$;

SELECT 'migration_039_verified' AS result;

ROLLBACK;
