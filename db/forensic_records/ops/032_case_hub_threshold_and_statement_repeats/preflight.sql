-- Read-only compatibility gate for migration 032 (case hub threshold, statement repeats across files). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  cols text;
BEGIN
  IF to_regclass('forensic.case_record') IS NULL THEN
    RAISE EXCEPTION 'migration 021 must be applied before migration 032 (forensic.case_record is missing)';
  END IF;
  IF to_regclass('forensic.bank_transaction') IS NULL OR to_regclass('analyst.v_bank_transactions') IS NULL THEN
    RAISE EXCEPTION 'migrations 029 and 030 must be applied before migration 032 (the bank transaction table or its view is missing)';
  END IF;
  SELECT string_agg(a.attname, ',' ORDER BY a.attnum) INTO cols FROM pg_attribute a
   WHERE a.attrelid = 'analyst.v_bank_transactions'::regclass AND a.attnum > 0 AND NOT a.attisdropped;
  IF cols IS DISTINCT FROM 'txn_id,evidence_id,source_row,account,counterparty_account,counterparty_msisdn,txn_utc,time_precision,direction,amount,signed_amount,currency,balance_after,channel,reference,status,time_basis,source_tz,balance_break' THEN
    RAISE EXCEPTION 'analyst.v_bank_transactions has other columns than migration 030 gave it (%): CREATE OR REPLACE VIEW would fail', cols;
  END IF;
  IF EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid = 'forensic.case_record'::regclass AND attname = 'hub_threshold' AND NOT attisdropped) THEN
    RAISE NOTICE 'forensic.case_record.hub_threshold exists already: the migration is idempotent and leaves it';
  END IF;
  RAISE NOTICE 'migration 032 preflight passed';
END
$$;

ROLLBACK;
