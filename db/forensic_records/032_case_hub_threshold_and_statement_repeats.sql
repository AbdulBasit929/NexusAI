-- Migration 032 (DRAFT, not applied): a hub threshold per case, and statement repeats across files, docs/data/case-setting-and-statement-repeats-20261031.md.
--
-- Why. Two weaknesses of the first live smoke of the new families (docs/data/live-window-20261030.md):
--
--   1. "Services and numbers that everyone contacts" are left out of link analysis by a threshold (a number in contact with more than N distinct subscribers is a hub). The views read it from the
--      setting app.hub_threshold (migration 017, 20 when unset) but the engine could only get it from one environment variable for every case. A case with 12 subscribers and a case with 400 are
--      not read at one threshold. This adds the case's own setting, forensic.case_record.hub_threshold (NULL = no setting: the engine default applies), set through PATCH /cases/{id} and read by
--      the Ask engine per question; a question that names a threshold itself uses its own number. The answer states the number used and where it came from.
--   2. A bank statement is often supplied twice: as the bank's CSV and as its PDF or Word rendition, or as two exports whose periods overlap. Loaded into one case they counted every transaction
--      twice (the renditions of the Orion case were kept in a collection of their own for that reason). analyst.v_bank_transactions now leaves out a row that another file of the same case
--      already holds with the same account, time, direction, amount and balance after (null-safe); the row that stays is the one of the file with the smaller evidence id, so the result does
--      not depend on the order of loading. Nothing is deleted: the repeat stays in forensic.bank_transaction with its source row, and the view says so in its description. Repeats WITHIN a file
--      were already flagged duplicate_row and left out; this does not change them.
--
-- A different transaction with the same account, time, direction, amount and balance in another file is indistinguishable from a repeat, and is treated as one; a statement without balances
-- compares on the other four fields only. That is stated in the description of the view and in the answer's limitation.
--
-- The view keeps its columns, their order and their types (the views over it, v_account_*, are untouched). Requires migrations 021 and 030. Idempotent. Apply by hand on a retained database:
-- ops/032_case_hub_threshold_and_statement_repeats/preflight.sql first, verify.sql after; rollback.sql restores the view of migration 030 and drops the column while no case has set it.
BEGIN;
SET LOCAL lock_timeout = '10s';

ALTER TABLE forensic.case_record ADD COLUMN IF NOT EXISTS hub_threshold integer;
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'case_record_hub_threshold_chk' AND conrelid = 'forensic.case_record'::regclass) THEN
    ALTER TABLE forensic.case_record ADD CONSTRAINT case_record_hub_threshold_chk CHECK (hub_threshold IS NULL OR hub_threshold BETWEEN 2 AND 1000);
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    GRANT UPDATE (hub_threshold) ON forensic.case_record TO forensic_runtime;
  END IF;
END
$$;
COMMENT ON COLUMN forensic.case_record.hub_threshold IS
  'The case''s threshold for "numbers and services that everyone contacts": a number in contact with more than this many distinct subscribers is a hub (2 to 1000). NULL means the case has no setting and the engine default (20) applies. A question that names a threshold uses its own number.';

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
  AND NOT ('duplicate_row' = ANY (t.quality_flags))
  AND NOT EXISTS (
    SELECT 1
    FROM forensic.bank_transaction o
    WHERE o.tenant_id = t.tenant_id
      AND o.collection_id = t.collection_id
      AND o.account_norm = t.account_norm
      AND o.txn_utc = t.txn_utc
      AND o.direction = t.direction
      AND o.amount = t.amount
      AND o.balance_after IS NOT DISTINCT FROM t.balance_after
      AND o.evidence_id < t.evidence_id
      AND NOT ('duplicate_row' = ANY (o.quality_flags))
  );

COMMENT ON VIEW analyst.v_bank_transactions IS
  'Transactions of bank statements and wallet lists, one row each (exact repeats within a file left out; a row that another file of the case already holds with the same account, time, direction, amount and balance is counted once, in the file with the smaller evidence id, and the repeat stays in the stored rows): the account, the other side, the direction, the amount (positive) and its signed form, the balance after, the channel, the reference and the time with its precision. Amounts are as the source states them. The narration and names are not held here.';

COMMIT;
