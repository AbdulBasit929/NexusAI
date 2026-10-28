-- Read-only compatibility gate for migration 029 (forensic.bank_transaction and the layout kind bank_transaction). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  has_rows boolean := false;
BEGIN
  IF to_regclass('forensic.layout_versions') IS NULL OR to_regclass('forensic.evidence_items') IS NULL THEN
    RAISE EXCEPTION 'migration 012 must be applied before migration 029 (forensic.layout_versions or evidence_items is missing)';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'timescaledb') THEN
    RAISE EXCEPTION 'the timescaledb extension is missing: forensic.bank_transaction is a hypertable like the other typed tables';
  END IF;
  IF to_regclass('forensic.bank_transaction') IS NOT NULL THEN
    EXECUTE 'SELECT EXISTS (SELECT 1 FROM forensic.bank_transaction)' INTO has_rows;
    IF has_rows THEN
      RAISE NOTICE 'forensic.bank_transaction exists and holds rows: the migration is idempotent and leaves them';
    END IF;
  END IF;
  IF EXISTS (SELECT 1 FROM forensic.layout_versions WHERE dataset_kind NOT IN ('target_cdr', 'tower_dump', 'ipdr', 'app_usage', 'subscriber', 'cell_reference', 'plate_sighting', 'bank_transaction', 'generic')) THEN
    RAISE EXCEPTION 'a registered layout has a dataset kind the new constraint would refuse';
  END IF;
  RAISE NOTICE 'migration 029 preflight passed';
END
$$;

ROLLBACK;
