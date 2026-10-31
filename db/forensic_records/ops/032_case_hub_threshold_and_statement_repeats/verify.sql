-- Read-only verification of migration 032. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  def text;
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid = 'forensic.case_record'::regclass AND attname = 'hub_threshold' AND NOT attisdropped AND atttypid = 'integer'::regtype) THEN
    RAISE EXCEPTION 'forensic.case_record.hub_threshold is missing or is not an integer (migration 032 is not applied)';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'case_record_hub_threshold_chk' AND conrelid = 'forensic.case_record'::regclass) THEN
    RAISE EXCEPTION 'the check constraint case_record_hub_threshold_chk is missing';
  END IF;
  IF col_description('forensic.case_record'::regclass, (SELECT attnum FROM pg_attribute WHERE attrelid = 'forensic.case_record'::regclass AND attname = 'hub_threshold')) IS NULL THEN
    RAISE EXCEPTION 'forensic.case_record.hub_threshold has no description';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime')
     AND has_column_privilege('forensic_runtime', 'forensic.case_record', 'hub_threshold', 'UPDATE') IS NOT TRUE THEN
    RAISE EXCEPTION 'forensic_runtime cannot update forensic.case_record.hub_threshold';
  END IF;
  IF EXISTS (SELECT 1 FROM forensic.case_record WHERE hub_threshold IS NOT NULL AND hub_threshold NOT BETWEEN 2 AND 1000) THEN
    RAISE EXCEPTION 'a case has a hub threshold outside 2 to 1000';
  END IF;

  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_bank_transactions'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'txn_id,evidence_id,source_row,account,counterparty_account,counterparty_msisdn,txn_utc,time_precision,direction,amount,signed_amount,currency,balance_after,channel,reference,status,time_basis,source_tz,balance_break' THEN
    RAISE EXCEPTION 'analyst.v_bank_transactions has other columns than before';
  END IF;
  def := pg_get_viewdef('analyst.v_bank_transactions'::regclass);
  IF def NOT LIKE '%app.tenant_id%' OR def NOT LIKE '%app.collection_id%' THEN
    RAISE EXCEPTION 'analyst.v_bank_transactions no longer carries its own tenant and collection scope';
  END IF;
  IF def NOT LIKE '%NOT (EXISTS%' OR def NOT LIKE '%evidence_id <%' OR def NOT LIKE '%duplicate_row%' THEN
    RAISE EXCEPTION 'analyst.v_bank_transactions does not leave out the repeats of another file (migration 032 is not applied)';
  END IF;
  IF def ~* '(narration|counterparty_label|raw_payload)' THEN
    RAISE EXCEPTION 'analyst.v_bank_transactions reads the narration, a name or the raw row: they stay out of every view';
  END IF;
  IF obj_description('analyst.v_bank_transactions'::regclass, 'pg_class') NOT LIKE '%another file of the case already holds%' THEN
    RAISE EXCEPTION 'the description of analyst.v_bank_transactions does not state the rule for repeats across files';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid = 'analyst.v_bank_transactions'::regclass AND a.attnum > 0 AND NOT a.attisdropped AND col_description(a.attrelid, a.attnum) IS NULL) THEN
    RAISE EXCEPTION 'a column of analyst.v_bank_transactions has no description';
  END IF;
  IF has_table_privilege('forensic_analyst_ro', 'analyst.v_bank_transactions'::regclass, 'SELECT') IS NOT TRUE THEN
    RAISE EXCEPTION 'forensic_analyst_ro cannot read analyst.v_bank_transactions';
  END IF;
  IF has_table_privilege('public', 'analyst.v_bank_transactions'::regclass, 'SELECT') THEN
    RAISE EXCEPTION 'PUBLIC can read analyst.v_bank_transactions';
  END IF;
  IF has_table_privilege('forensic_analyst_ro', 'forensic.bank_transaction', 'SELECT') THEN
    RAISE EXCEPTION 'forensic_analyst_ro can read forensic.bank_transaction directly: it may read only through the views';
  END IF;
  IF has_table_privilege('forensic_analyst_ro', 'forensic.case_record', 'SELECT') THEN
    RAISE EXCEPTION 'forensic_analyst_ro can read forensic.case_record directly: the engine reads the case setting with the runtime role';
  END IF;
  RAISE NOTICE 'migration_032_verified';
END
$$;

ROLLBACK;
