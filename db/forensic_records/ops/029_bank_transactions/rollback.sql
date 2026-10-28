-- Rollback for 029_bank_transactions.sql: drops forensic.bank_transaction (only while it is empty) and restores the old layout kind constraint. Roll migration 030 (the views) back first.
-- A table that holds rows is NEVER dropped: the whole script is one transaction that stops with a message and changes nothing, and the rows are exported and reviewed first. Likewise a registered
-- layout of kind bank_transaction stops the constraint from being restored.
BEGIN;
DO $$
DECLARE
  has_rows boolean := false;
BEGIN
  IF to_regclass('analyst.v_bank_transactions') IS NOT NULL THEN
    RAISE EXCEPTION 'migration 030 (the transaction views) is still applied: nothing was changed; roll it back first';
  END IF;
  IF to_regclass('forensic.bank_transaction') IS NOT NULL THEN
    EXECUTE 'SELECT EXISTS (SELECT 1 FROM forensic.bank_transaction)' INTO has_rows;
    IF has_rows THEN
      RAISE EXCEPTION 'forensic.bank_transaction holds rows: nothing was changed; export and review them before a rollback';
    END IF;
  END IF;
  IF EXISTS (SELECT 1 FROM forensic.layout_versions WHERE dataset_kind = 'bank_transaction') THEN
    RAISE EXCEPTION 'a layout of kind bank_transaction is registered: nothing was changed; retire it before a rollback';
  END IF;
END
$$;
DROP TABLE IF EXISTS forensic.bank_transaction;
ALTER TABLE forensic.layout_versions DROP CONSTRAINT IF EXISTS layout_versions_kind_chk;
ALTER TABLE forensic.layout_versions ADD CONSTRAINT layout_versions_kind_chk CHECK (dataset_kind IN
  ('target_cdr', 'tower_dump', 'ipdr', 'app_usage', 'subscriber', 'cell_reference', 'plate_sighting', 'generic'));
COMMIT;
