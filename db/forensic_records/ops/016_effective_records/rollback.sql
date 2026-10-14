-- Rollback for 016_effective_records.sql: drops the view. Nothing else: no table, row or policy was changed by the migration, so
-- nothing is lost. Whatever read the view (the question lane with FORENSIC_TYPED_FAMILIES set) must be
-- switched back first (unset the variable and recreate the API container), or its queries on that family fail until it is.
BEGIN;
DROP VIEW IF EXISTS forensic.records_effective_cdr;
COMMIT;
