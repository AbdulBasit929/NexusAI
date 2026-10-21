-- Read-only verification of migration 021. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;
DO $$
DECLARE
  problem text;
BEGIN
  IF to_regclass('forensic.case_record') IS NULL THEN
    RAISE EXCEPTION 'forensic.case_record is missing';
  END IF;
  SELECT string_agg(c.name, ', ') INTO problem
  FROM (VALUES ('tenant_id'), ('collection_id'), ('display_name'), ('status'), ('created_by'), ('created_at'), ('updated_by'), ('updated_at')) AS c(name)
  WHERE NOT EXISTS (SELECT 1 FROM information_schema.columns k WHERE k.table_schema = 'forensic' AND k.table_name = 'case_record' AND k.column_name = c.name);
  IF problem IS NOT NULL THEN RAISE EXCEPTION 'missing columns: %', problem; END IF;

  SELECT string_agg(c.conname, ', ') INTO problem
  FROM (VALUES ('case_record_status_chk'), ('case_record_collection_chk'), ('case_record_name_chk')) AS c(conname)
  WHERE NOT EXISTS (SELECT 1 FROM pg_constraint k WHERE k.conrelid = 'forensic.case_record'::regclass AND k.conname = c.conname AND k.convalidated);
  IF problem IS NOT NULL THEN RAISE EXCEPTION 'missing or unvalidated constraints: %', problem; END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_class WHERE oid = 'forensic.case_record'::regclass AND relrowsecurity) THEN
    RAISE EXCEPTION 'row-level security is not enabled on forensic.case_record';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = 'case_record' AND policyname = 'tenant_isolation_case_record'
                 AND cmd = 'ALL' AND permissive = 'PERMISSIVE' AND qual IS NOT NULL AND with_check IS NOT NULL) THEN
    RAISE EXCEPTION 'the tenant policy of forensic.case_record is missing or altered';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    IF NOT has_table_privilege('forensic_runtime', 'forensic.case_record', 'SELECT') OR NOT has_table_privilege('forensic_runtime', 'forensic.case_record', 'INSERT') THEN
      RAISE EXCEPTION 'forensic_runtime lacks SELECT or INSERT on forensic.case_record';
    END IF;
    IF has_table_privilege('forensic_runtime', 'forensic.case_record', 'DELETE') THEN
      RAISE EXCEPTION 'forensic_runtime may delete case records; it must not';
    END IF;
    IF has_column_privilege('forensic_runtime', 'forensic.case_record', 'collection_id', 'UPDATE') OR has_column_privilege('forensic_runtime', 'forensic.case_record', 'created_by', 'UPDATE') THEN
      RAISE EXCEPTION 'forensic_runtime may change the identity or creator columns of a case record; it must not';
    END IF;
    IF NOT has_column_privilege('forensic_runtime', 'forensic.case_record', 'display_name', 'UPDATE') OR NOT has_column_privilege('forensic_runtime', 'forensic.case_record', 'status', 'UPDATE') THEN
      RAISE EXCEPTION 'forensic_runtime cannot update display_name or status';
    END IF;
  END IF;
  RAISE NOTICE 'migration 021 verified';
END
$$;
ROLLBACK;
