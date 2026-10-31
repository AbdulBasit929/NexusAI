-- Rollback for 032_case_hub_threshold_and_statement_repeats.sql: restores analyst.v_bank_transactions as migration 030 defined it (every stored row counts again, repeats across files included) and drops
-- forensic.case_record.hub_threshold, but only while no case has set it. A column that holds settings is NEVER dropped: the whole script is one transaction that stops with a message and changes nothing;
-- the settings are exported and reviewed first (SELECT collection_id, hub_threshold FROM forensic.case_record WHERE hub_threshold IS NOT NULL).
-- An Ask engine that reads the case setting must be rolled back with it, or its read of the column fails and the engine falls back to the default threshold for every case.
BEGIN;
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid = 'forensic.case_record'::regclass AND attname = 'hub_threshold' AND NOT attisdropped) THEN
    IF EXISTS (SELECT 1 FROM forensic.case_record WHERE hub_threshold IS NOT NULL) THEN
      RAISE EXCEPTION 'a case has set its hub threshold: nothing was changed; export and review the settings before a rollback';
    END IF;
  END IF;
END
$$;

CREATE OR REPLACE VIEW analyst.v_bank_transactions AS
SELECT
  t.txn_id,
  t.evidence_id,
  t.source_row,
  t.account_norm AS account,
  t.counterparty_account_norm AS counterparty_account,
  t.counterparty_msisdn,
  t.txn_utc,
  t.time_precision,
  t.direction,
  t.amount,
  CASE WHEN t.direction = 'credit' THEN t.amount ELSE -t.amount END AS signed_amount,
  t.currency,
  t.balance_after,
  t.channel,
  t.reference,
  t.status,
  t.time_basis,
  t.source_tz,
  ('balance_break' = ANY (t.quality_flags)) AS balance_break
FROM forensic.bank_transaction t
WHERE t.tenant_id = current_setting('app.tenant_id', true)
  AND t.collection_id = current_setting('app.collection_id', true)
  AND NOT ('duplicate_row' = ANY (t.quality_flags));

COMMENT ON VIEW analyst.v_bank_transactions IS
  'Transactions of bank statements and wallet lists, one row each (exact repeats within a file left out): the account, the other side, the direction, the amount (positive) and its signed form, the balance after, the channel, the reference and the time with its precision. Amounts are as the source states them. The narration and names are not held here.';

ALTER TABLE forensic.case_record DROP CONSTRAINT IF EXISTS case_record_hub_threshold_chk;
ALTER TABLE forensic.case_record DROP COLUMN IF EXISTS hub_threshold;
COMMIT;
