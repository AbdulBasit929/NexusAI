-- Read-only verification of migration 029. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;

DO $$
BEGIN
  IF to_regclass('forensic.bank_transaction') IS NULL THEN
    RAISE EXCEPTION 'forensic.bank_transaction is missing (migration 029 is not applied)';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM timescaledb_information.hypertables WHERE hypertable_schema = 'forensic' AND hypertable_name = 'bank_transaction') THEN
    RAISE EXCEPTION 'forensic.bank_transaction is not a hypertable';
  END IF;
  IF NOT (SELECT relrowsecurity FROM pg_class WHERE oid = 'forensic.bank_transaction'::regclass) THEN
    RAISE EXCEPTION 'row-level security is off on forensic.bank_transaction';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = 'bank_transaction' AND policyname = 'tenant_isolation_bank_transaction') THEN
    RAISE EXCEPTION 'the tenant policy of forensic.bank_transaction is missing';
  END IF;
  IF pg_get_constraintdef((SELECT oid FROM pg_constraint WHERE conname = 'layout_versions_kind_chk' AND conrelid = 'forensic.layout_versions'::regclass)) NOT LIKE '%bank_transaction%' THEN
    RAISE EXCEPTION 'the layout kind constraint does not allow bank_transaction';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_analyst_ro') AND has_table_privilege('forensic_analyst_ro', 'forensic.bank_transaction', 'SELECT') THEN
    RAISE EXCEPTION 'forensic_analyst_ro can read forensic.bank_transaction directly: it may read only through the views';
  END IF;
  RAISE NOTICE 'migration_029_verified';
END
$$;

ROLLBACK;
