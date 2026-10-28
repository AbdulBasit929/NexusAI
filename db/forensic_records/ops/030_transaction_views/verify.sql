-- Read-only verification of migration 030. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  v text;
BEGIN
  FOREACH v IN ARRAY ARRAY['v_bank_transactions', 'v_account_counterparties', 'v_account_activity', 'v_account_flows', 'v_account_limit_bands'] LOOP
    IF to_regclass('analyst.' || v) IS NULL THEN
      RAISE EXCEPTION 'analyst.% is missing (migration 030 is not applied)', v;
    END IF;
    IF obj_description(('analyst.' || v)::regclass, 'pg_class') IS NULL THEN
      RAISE EXCEPTION 'analyst.% has no description', v;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid = ('analyst.' || v)::regclass AND a.attnum > 0 AND NOT a.attisdropped AND col_description(a.attrelid, a.attnum) IS NULL) THEN
      RAISE EXCEPTION 'a column of analyst.% has no description', v;
    END IF;
    IF has_table_privilege('forensic_analyst_ro', ('analyst.' || v)::regclass, 'SELECT') IS NOT TRUE THEN
      RAISE EXCEPTION 'forensic_analyst_ro cannot read analyst.%', v;
    END IF;
    IF has_table_privilege('public', ('analyst.' || v)::regclass, 'SELECT') THEN
      RAISE EXCEPTION 'PUBLIC can read analyst.%', v;
    END IF;
    IF pg_get_viewdef(('analyst.' || v)::regclass) ~* '(narration|counterparty_label|raw_payload)' THEN
      RAISE EXCEPTION 'analyst.% reads the narration, a name or the raw row: they stay out of every view', v;
    END IF;
  END LOOP;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_bank_transactions'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'txn_id,evidence_id,source_row,account,counterparty_account,counterparty_msisdn,txn_utc,time_precision,direction,amount,signed_amount,currency,balance_after,channel,reference,status,time_basis,source_tz,balance_break' THEN
    RAISE EXCEPTION 'analyst.v_bank_transactions has other columns than the migration gave it';
  END IF;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_account_flows'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'account,credit_id,credit_utc,credit_amount,received_from,debit_id,debit_utc,debit_amount,sent_to,minutes_between,share_percent' THEN
    RAISE EXCEPTION 'analyst.v_account_flows has other columns than the migration gave it';
  END IF;
  IF pg_get_viewdef('analyst.v_bank_transactions'::regclass) NOT LIKE '%app.tenant_id%' OR pg_get_viewdef('analyst.v_bank_transactions'::regclass) NOT LIKE '%app.collection_id%' THEN
    RAISE EXCEPTION 'analyst.v_bank_transactions no longer carries its own tenant and collection scope';
  END IF;
  FOREACH v IN ARRAY ARRAY['v_account_counterparties', 'v_account_activity', 'v_account_flows', 'v_account_limit_bands'] LOOP
    IF pg_get_viewdef(('analyst.' || v)::regclass) NOT LIKE '%v_bank_transactions%' THEN
      RAISE EXCEPTION 'analyst.% does not read analyst.v_bank_transactions (it takes its scope from it)', v;
    END IF;
  END LOOP;
  IF pg_get_viewdef('analyst.v_account_flows'::regclass) NOT LIKE '%app.flow_window_minutes%' OR pg_get_viewdef('analyst.v_account_flows'::regclass) NOT LIKE '%app.flow_share_percent%' THEN
    RAISE EXCEPTION 'analyst.v_account_flows no longer reads its runtime parameters';
  END IF;
  IF has_table_privilege('forensic_analyst_ro', 'forensic.bank_transaction', 'SELECT') THEN
    RAISE EXCEPTION 'forensic_analyst_ro can read forensic.bank_transaction directly: it may read only through the views';
  END IF;
  RAISE NOTICE 'migration_030_verified';
END
$$;

ROLLBACK;
