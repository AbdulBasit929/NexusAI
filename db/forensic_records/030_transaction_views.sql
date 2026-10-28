-- Migration 030 (DRAFT, not applied): analyst views over the typed bank and wallet transactions of migration 029, docs/data/transactions-family-20261028.md (P1-B, fourth family).
--
-- Why. "How much came in and went out of this account", "who did it pay", "which accounts share a counterparty", "money in and almost the same amount out within minutes" are ordinary questions of a
-- financial inquiry; the Ask engine reaches data only through reviewed analyst views (migration 017), so this adds five.
--
--   * analyst.v_bank_transactions         one row per transaction (exact repeats within a file left out): account, counterparty, direction, amount, balance after, channel, reference, time and its precision;
--   * analyst.v_account_counterparties    per account, counterparty and direction: how many, how much, first and last;
--   * analyst.v_account_activity          per account and local day: credits, debits, count, the balance after the day's last row;
--   * analyst.v_account_flows             pass-through candidates: a credit and a later debit of the same account of at least a share of it within a window; the share (default 90 percent) and the window
--                                         (default 10 minutes) are runtime parameters read from the settings app.flow_share_percent and app.flow_window_minutes;
--   * analyst.v_account_limit_bands       per account: debits at or above a share of a limit and below it; the limit and the share are runtime parameters too (app.limit_amount,
--                                         app.limit_share_percent, default share 95 percent); with no limit set the view has no rows.
--
-- A transaction with time_precision 'date' (only the day is known) must not answer a question about minutes: the flow view leaves such rows out. The narration, the counterparty's name and the raw row are
-- NOT in any view (they can name a person; the masking design, roadmap R9, is pending). A pattern of rows (money in and out within minutes, amounts just under a limit) is a pattern, not a finding.
--
-- Scope is STRUCTURAL, as in migration 017: each view takes `tenant_id = current_setting('app.tenant_id', true) AND collection_id = current_setting('app.collection_id', true)`; an unset setting is NULL and the
-- view returns no rows (fail closed).
--
-- Requires migrations 017 and 029. Idempotent. Apply by hand on a retained database: ops/030_transaction_views/preflight.sql first, verify.sql after; rollback.sql drops the views.
BEGIN;
SET LOCAL lock_timeout = '10s';

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

CREATE OR REPLACE VIEW analyst.v_account_counterparties AS
SELECT
  b.account,
  coalesce(b.counterparty_account, b.counterparty_msisdn) AS counterparty,
  b.direction,
  count(*) AS transactions,
  sum(b.amount) AS total_amount,
  min(b.txn_utc) AS first_utc,
  max(b.txn_utc) AS last_utc
FROM analyst.v_bank_transactions b
WHERE coalesce(b.counterparty_account, b.counterparty_msisdn) IS NOT NULL
GROUP BY b.account, coalesce(b.counterparty_account, b.counterparty_msisdn), b.direction;

CREATE OR REPLACE VIEW analyst.v_account_activity AS
SELECT
  b.account,
  (b.txn_utc AT TIME ZONE current_setting('TimeZone'))::date AS day,
  count(*) AS transactions,
  coalesce(sum(b.amount) FILTER (WHERE b.direction = 'credit'), 0) AS credits,
  coalesce(sum(b.amount) FILTER (WHERE b.direction = 'debit'), 0) AS debits,
  (array_agg(b.balance_after ORDER BY b.txn_utc DESC, b.txn_id DESC) FILTER (WHERE b.balance_after IS NOT NULL))[1] AS closing_balance
FROM analyst.v_bank_transactions b
GROUP BY b.account, (b.txn_utc AT TIME ZONE current_setting('TimeZone'))::date;

CREATE OR REPLACE VIEW analyst.v_account_flows AS
SELECT
  c.account,
  c.txn_id AS credit_id,
  c.txn_utc AS credit_utc,
  c.amount AS credit_amount,
  coalesce(c.counterparty_account, c.counterparty_msisdn) AS received_from,
  d.txn_id AS debit_id,
  d.txn_utc AS debit_utc,
  d.amount AS debit_amount,
  coalesce(d.counterparty_account, d.counterparty_msisdn) AS sent_to,
  round(extract(epoch FROM (d.txn_utc - c.txn_utc)) / 60.0, 1) AS minutes_between,
  round(100.0 * d.amount / c.amount, 1) AS share_percent
FROM analyst.v_bank_transactions c
JOIN analyst.v_bank_transactions d
  ON d.account = c.account
 AND c.direction = 'credit' AND d.direction = 'debit'
 AND d.txn_utc > c.txn_utc
 AND d.txn_utc <= c.txn_utc + make_interval(mins => coalesce(nullif(current_setting('app.flow_window_minutes', true), '')::int, 10))
 AND d.amount <= c.amount
 AND d.amount >= c.amount * coalesce(nullif(current_setting('app.flow_share_percent', true), '')::numeric, 90) / 100.0
WHERE c.time_precision <> 'date' AND d.time_precision <> 'date' AND c.amount > 0;

CREATE OR REPLACE VIEW analyst.v_account_limit_bands AS
SELECT
  b.account,
  count(*) AS transactions,
  sum(b.amount) AS total_amount,
  min(b.txn_utc) AS first_utc,
  max(b.txn_utc) AS last_utc,
  nullif(current_setting('app.limit_amount', true), '')::numeric AS limit_amount,
  coalesce(nullif(current_setting('app.limit_share_percent', true), '')::numeric, 95) AS share_percent
FROM analyst.v_bank_transactions b
WHERE b.direction = 'debit'
  AND nullif(current_setting('app.limit_amount', true), '') IS NOT NULL
  AND b.amount >= nullif(current_setting('app.limit_amount', true), '')::numeric * coalesce(nullif(current_setting('app.limit_share_percent', true), '')::numeric, 95) / 100.0
  AND b.amount < nullif(current_setting('app.limit_amount', true), '')::numeric
GROUP BY b.account;

COMMENT ON VIEW analyst.v_bank_transactions IS
  'Transactions of bank statements and wallet lists, one row each (exact repeats within a file left out): the account, the other side, the direction, the amount (positive) and its signed form, the balance after, the channel, the reference and the time with its precision. Amounts are as the source states them. The narration and names are not held here.';
COMMENT ON VIEW analyst.v_account_counterparties IS
  'Per account, counterparty (an account or a mobile number) and direction: how many transactions, how much, the first and the last. For who an account paid, who paid it and which accounts share a counterparty. A shared counterparty is a pattern in the rows, not a finding.';
COMMENT ON VIEW analyst.v_account_activity IS
  'Per account and local day (the case clock): the number of transactions, the credits, the debits and the balance after the last row of the day. For totals over a period and for the balance on a date.';
COMMENT ON VIEW analyst.v_account_flows IS
  'Pass-through candidates: a credit and a later debit of the same account within the window (runtime parameter, default 10 minutes) of at least the share (runtime parameter, default 90 percent) of it, only from rows that state a time of day. A pattern in the rows, not a finding: the records do not say why money moved on.';
COMMENT ON VIEW analyst.v_account_limit_bands IS
  'Per account: debits at or above a share (default 95 percent) of a stated limit and below it; needs the limit of the question (runtime parameter app.limit_amount), none set gives no rows. A pattern in the rows, not a finding.';

DO $$
DECLARE
  d record;
BEGIN
  FOR d IN
    SELECT * FROM (VALUES
      ('v_bank_transactions', 'txn_id', 'Identifier of the transaction row.'),
      ('v_bank_transactions', 'evidence_id', 'The evidence file the transaction came from.'),
      ('v_bank_transactions', 'source_row', 'The row of that file.'),
      ('v_bank_transactions', 'account', 'The account (or wallet number) the transaction is of, as a search key: capitals, no spaces or dashes. Always compare this column, written the same way.'),
      ('v_bank_transactions', 'counterparty_account', 'The other account of the transaction, as a search key, when the source states one.'),
      ('v_bank_transactions', 'counterparty_msisdn', 'The other side''s mobile number (92 and ten digits) when the source states one, as for a wallet transfer: the link to the call records.'),
      ('v_bank_transactions', 'txn_utc', 'Time of the transaction, UTC (shown on the case clock). For a row with time_precision date it is local midnight of the day.'),
      ('v_bank_transactions', 'time_precision', 'second or minute: the source states a time of day; date: only the day. A question about minutes or hours must not use a date row.'),
      ('v_bank_transactions', 'direction', 'debit: money left the account; credit: money came into it.'),
      ('v_bank_transactions', 'amount', 'The amount, always positive, as the source states it (no conversion). The direction says which way it went.'),
      ('v_bank_transactions', 'signed_amount', 'The amount with its sign: credits positive, debits negative. Add it up for the net movement.'),
      ('v_bank_transactions', 'currency', 'The currency the source states, when it states one; empty otherwise (the amount is not converted).'),
      ('v_bank_transactions', 'balance_after', 'The account balance after this transaction as the source states it, when it has one.'),
      ('v_bank_transactions', 'channel', 'How the transaction was made: ibft, raast, atm, pos, cheque, cash, wallet, branch, online or other.'),
      ('v_bank_transactions', 'reference', 'The source''s own reference or transaction id, when it has one.'),
      ('v_bank_transactions', 'status', 'The transaction status the source states (completed, failed), when it has one.'),
      ('v_bank_transactions', 'time_basis', 'How the zone of the time is known: declared, assumed_by_profile, confirmed_by_analyst or explicit_offset.'),
      ('v_bank_transactions', 'source_tz', 'The time zone the source''s times were read in.'),
      ('v_bank_transactions', 'balance_break', 'True when the balance after this row does not follow from the previous row of the same account: a row may be missing or the statement altered. The row is kept.'),
      ('v_account_counterparties', 'account', 'The account (a search key).'),
      ('v_account_counterparties', 'counterparty', 'The other side: an account or a mobile number, as a search key.'),
      ('v_account_counterparties', 'direction', 'debit: the account paid the counterparty; credit: the counterparty paid the account.'),
      ('v_account_counterparties', 'transactions', 'How many transactions of that direction between the two.'),
      ('v_account_counterparties', 'total_amount', 'The total of those amounts, as stated (no conversion).'),
      ('v_account_counterparties', 'first_utc', 'Time of the first of them, UTC.'),
      ('v_account_counterparties', 'last_utc', 'Time of the last of them, UTC.'),
      ('v_account_activity', 'account', 'The account (a search key).'),
      ('v_account_activity', 'day', 'The local day on the case clock.'),
      ('v_account_activity', 'transactions', 'How many transactions the account had that day.'),
      ('v_account_activity', 'credits', 'The total that came in that day.'),
      ('v_account_activity', 'debits', 'The total that went out that day.'),
      ('v_account_activity', 'closing_balance', 'The balance after the last row of the day that states one; empty when no row of the day does.'),
      ('v_account_flows', 'account', 'The account (a search key).'),
      ('v_account_flows', 'credit_id', 'Identifier of the credit row (v_bank_transactions.txn_id).'),
      ('v_account_flows', 'credit_utc', 'Time of the credit, UTC.'),
      ('v_account_flows', 'credit_amount', 'The amount that came in.'),
      ('v_account_flows', 'received_from', 'Who it came from: an account or a mobile number, when the source states one.'),
      ('v_account_flows', 'debit_id', 'Identifier of the later debit row.'),
      ('v_account_flows', 'debit_utc', 'Time of the debit, UTC.'),
      ('v_account_flows', 'debit_amount', 'The amount that went out.'),
      ('v_account_flows', 'sent_to', 'Who it went to: an account or a mobile number, when the source states one.'),
      ('v_account_flows', 'minutes_between', 'Minutes from the credit to the debit.'),
      ('v_account_flows', 'share_percent', 'The debit as a percent of the credit.'),
      ('v_account_limit_bands', 'account', 'The account (a search key).'),
      ('v_account_limit_bands', 'transactions', 'How many debits fall at or above the share of the limit and below it.'),
      ('v_account_limit_bands', 'total_amount', 'The total of those debits.'),
      ('v_account_limit_bands', 'first_utc', 'Time of the first of them, UTC.'),
      ('v_account_limit_bands', 'last_utc', 'Time of the last of them, UTC.'),
      ('v_account_limit_bands', 'limit_amount', 'The limit of the question (runtime parameter).'),
      ('v_account_limit_bands', 'share_percent', 'The share of the limit that starts the band (runtime parameter, default 95).')
    ) AS t(view_name, column_name, description)
  LOOP
    EXECUTE format('COMMENT ON COLUMN analyst.%I.%I IS %L', d.view_name, d.column_name, d.description);
  END LOOP;
END
$$;

REVOKE ALL ON analyst.v_bank_transactions, analyst.v_account_counterparties, analyst.v_account_activity, analyst.v_account_flows, analyst.v_account_limit_bands FROM PUBLIC;
GRANT SELECT ON analyst.v_bank_transactions, analyst.v_account_counterparties, analyst.v_account_activity, analyst.v_account_flows, analyst.v_account_limit_bands TO forensic_analyst_ro;
COMMIT;
