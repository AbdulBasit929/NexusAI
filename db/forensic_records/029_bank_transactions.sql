-- Migration 029 (DRAFT, not applied): the typed table of bank and wallet transactions, docs/data/transactions-family-20261028.md (P1-B, fourth family). The analyst views over it are migration 030.
--
-- Why. A statement or a wallet list is a list of posted transactions, and no typed table holds them. This adds the layout kind `bank_transaction` (the CHECK of forensic.layout_versions is replaced by one
-- that also allows it) and forensic.bank_transaction, filled by the production typed ingest from the layouts bank-stmt-t1 and wallet-t1 and later layouts: a hypertable on the transaction time, tenant
-- row-level security, INSERT and SELECT for the runtime role, like the other typed tables. It needs only migration 012, so that layouts of the new kind can be registered early.
--
-- Amounts are as the source states them (positive, with a direction), with no conversion. A transaction posted with a date only has time_precision 'date' (local midnight). The narration, the
-- counterparty's name and the raw row are the source's own text: stored with the row and exposed by no view (migration 030).
--
-- Requires migration 012 and the timescaledb extension. Idempotent. Apply by hand on a retained database: ops/029_bank_transactions/preflight.sql first, verify.sql after; rollback.sql drops the table
-- when it is empty.
BEGIN;
SET LOCAL lock_timeout = '10s';

ALTER TABLE forensic.layout_versions DROP CONSTRAINT IF EXISTS layout_versions_kind_chk;
ALTER TABLE forensic.layout_versions ADD CONSTRAINT layout_versions_kind_chk CHECK (dataset_kind IN
  ('target_cdr', 'tower_dump', 'ipdr', 'app_usage', 'subscriber', 'cell_reference', 'plate_sighting', 'bank_transaction', 'generic'));

CREATE TABLE IF NOT EXISTS forensic.bank_transaction (
  txn_id bigint GENERATED ALWAYS AS IDENTITY,
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  evidence_id uuid NOT NULL,
  file_id text NOT NULL,
  batch_id uuid NOT NULL,
  source_row bigint NOT NULL,
  record_id uuid,
  record_ts timestamptz,
  row_hash text NOT NULL,
  content_hash text NOT NULL,
  layout_id text NOT NULL,
  layout_version integer NOT NULL,
  txn_utc timestamptz NOT NULL,
  time_basis text NOT NULL,
  time_precision text NOT NULL DEFAULT 'second',
  source_tz text,
  account_raw text NOT NULL,
  account_norm text NOT NULL,
  counterparty_account_raw text,
  counterparty_account_norm text,
  counterparty_msisdn text,
  counterparty_label text,
  direction text NOT NULL,
  amount numeric(18, 2) NOT NULL,
  currency text,
  balance_after numeric(18, 2),
  channel text,
  reference text,
  narration text,
  status text,
  raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  quality_flags text[] NOT NULL DEFAULT '{}',
  ingested_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (txn_id, txn_utc),
  UNIQUE (tenant_id, collection_id, file_id, row_hash, txn_utc),
  CONSTRAINT bank_transaction_layout_fk FOREIGN KEY (layout_id, layout_version) REFERENCES forensic.layout_versions (layout_id, layout_version),
  CONSTRAINT bank_transaction_evidence_fk FOREIGN KEY (tenant_id, collection_id, evidence_id)
    REFERENCES forensic.evidence_items (tenant_id, collection_id, evidence_id),
  CONSTRAINT bank_transaction_payload_chk CHECK (jsonb_typeof(raw_payload) = 'object'),
  CONSTRAINT bank_transaction_direction_chk CHECK (direction IN ('debit', 'credit')),
  CONSTRAINT bank_transaction_amount_chk CHECK (amount >= 0),
  CONSTRAINT bank_transaction_precision_chk CHECK (time_precision IN ('date', 'minute', 'second')),
  CONSTRAINT bank_transaction_time_basis_chk CHECK (time_basis IN ('declared', 'assumed_by_profile', 'confirmed_by_analyst', 'explicit_offset')),
  CONSTRAINT bank_transaction_channel_chk CHECK (channel IS NULL OR channel IN ('ibft', 'raast', 'atm', 'pos', 'cheque', 'cash', 'wallet', 'branch', 'online', 'other')),
  CONSTRAINT bank_transaction_account_chk CHECK (account_norm <> '')
);

SELECT create_hypertable('forensic.bank_transaction', 'txn_utc', if_not_exists => TRUE, chunk_time_interval => INTERVAL '7 days');

CREATE INDEX IF NOT EXISTS bank_transaction_account_time_idx ON forensic.bank_transaction (tenant_id, collection_id, account_norm, txn_utc DESC);
CREATE INDEX IF NOT EXISTS bank_transaction_counterparty_idx ON forensic.bank_transaction (tenant_id, collection_id, counterparty_account_norm, txn_utc DESC) WHERE counterparty_account_norm IS NOT NULL;
CREATE INDEX IF NOT EXISTS bank_transaction_msisdn_idx ON forensic.bank_transaction (tenant_id, collection_id, counterparty_msisdn, txn_utc DESC) WHERE counterparty_msisdn IS NOT NULL;

COMMENT ON TABLE forensic.bank_transaction IS
  'Posted transactions of bank statements and wallet lists, as the source states them (amounts positive, direction debit or credit). narration, counterparty_label and raw_payload are the source''s own text and are not exposed by any analyst view.';
COMMENT ON COLUMN forensic.bank_transaction.time_precision IS 'second or minute: the source states a time of day; date: only the day (stored at local midnight); a question about minutes must not read a date row.';

ALTER TABLE forensic.bank_transaction ENABLE ROW LEVEL SECURITY;
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = 'bank_transaction' AND policyname = 'tenant_isolation_bank_transaction') THEN
    CREATE POLICY tenant_isolation_bank_transaction ON forensic.bank_transaction USING (tenant_id = current_setting('app.tenant_id', true)) WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    GRANT SELECT, INSERT ON forensic.bank_transaction TO forensic_runtime;
  END IF;
END
$$;

COMMIT;
