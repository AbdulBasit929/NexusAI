-- Behaviour test of the repeat rule of migration 032 on whatever bank transactions the database holds. It WRITES inside one transaction and ALWAYS rolls back: nothing stays.
-- Run as the migrator (psql -v ON_ERROR_STOP=1). It needs one collection whose bank transactions come from at least two different files; with none it says so and passes nothing.
--   1. copy 5 rows of one file under the evidence id of a second file of the same collection (as a PDF rendition of the same statement would be);
--   2. the stored rows grow by 5, the view does not (the repeats are counted once);
--   3. a copy with a different amount is NOT a repeat and appears in the view;
--   4. a copy under the SAME file is left to the old rule (it is not hidden by this one).
BEGIN;

DO $$
DECLARE
  t text; c text; ev_a uuid; ev_b uuid; acct text;
  stored_before bigint; stored_after bigint; view_before bigint; view_after bigint;
  n int;
BEGIN
  SELECT b.tenant_id, b.collection_id INTO t, c
    FROM forensic.bank_transaction b
   GROUP BY b.tenant_id, b.collection_id
  HAVING count(DISTINCT b.evidence_id) >= 2
   ORDER BY count(*) DESC LIMIT 1;
  IF t IS NULL THEN
    RAISE EXCEPTION 'behaviour test needs a collection with bank transactions from at least two files; none exists here';
  END IF;
  PERFORM set_config('app.tenant_id', t, true);
  PERFORM set_config('app.collection_id', c, true);
  SELECT evidence_id INTO ev_a FROM forensic.bank_transaction WHERE tenant_id = t AND collection_id = c GROUP BY evidence_id ORDER BY count(*) DESC LIMIT 1;
  SELECT evidence_id INTO ev_b FROM forensic.bank_transaction WHERE tenant_id = t AND collection_id = c AND evidence_id <> ev_a GROUP BY evidence_id ORDER BY count(*) DESC LIMIT 1;
  SELECT count(*) INTO stored_before FROM forensic.bank_transaction WHERE tenant_id = t AND collection_id = c;
  EXECUTE 'SELECT count(*) FROM analyst.v_bank_transactions' INTO view_before;

  -- 1 and 2: five rows of file A copied under file B
  INSERT INTO forensic.bank_transaction (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, record_id, record_ts, row_hash, content_hash, layout_id, layout_version,
         txn_utc, time_basis, time_precision, source_tz, account_raw, account_norm, counterparty_account_raw, counterparty_account_norm, counterparty_msisdn, counterparty_label,
         direction, amount, currency, balance_after, channel, reference, narration, status, raw_payload, quality_flags)
  SELECT tenant_id, collection_id, ev_b, 'behaviour-test-b', batch_id, 100000 + source_row, record_id, record_ts, 'bt-' || md5(row_hash), content_hash, layout_id, layout_version,
         txn_utc, time_basis, time_precision, source_tz, account_raw, account_norm, counterparty_account_raw, counterparty_account_norm, counterparty_msisdn, counterparty_label,
         direction, amount, currency, balance_after, channel, reference, narration, status, raw_payload, quality_flags
    FROM forensic.bank_transaction
   WHERE tenant_id = t AND collection_id = c AND evidence_id = ev_a AND NOT ('duplicate_row' = ANY (quality_flags))
   ORDER BY source_row LIMIT 5;
  GET DIAGNOSTICS n = ROW_COUNT;
  IF n <> 5 THEN RAISE EXCEPTION 'could not copy 5 rows (copied %)', n; END IF;
  SELECT count(*) INTO stored_after FROM forensic.bank_transaction WHERE tenant_id = t AND collection_id = c;
  EXECUTE 'SELECT count(*) FROM analyst.v_bank_transactions' INTO view_after;
  IF stored_after <> stored_before + 5 THEN RAISE EXCEPTION 'the stored rows did not grow by 5 (% -> %)', stored_before, stored_after; END IF;
  IF view_after <> view_before THEN RAISE EXCEPTION 'repeats of another file are counted twice: the view went from % to % rows', view_before, view_after; END IF;
  RAISE NOTICE 'repeat across files counted once: stored % -> %, view % -> % (collection %)', stored_before, stored_after, view_before, view_after, c;

  -- 3: a copy with another amount is a different transaction
  INSERT INTO forensic.bank_transaction (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, record_id, record_ts, row_hash, content_hash, layout_id, layout_version,
         txn_utc, time_basis, time_precision, source_tz, account_raw, account_norm, direction, amount, currency, balance_after, raw_payload, quality_flags)
  SELECT tenant_id, collection_id, ev_b, 'behaviour-test-b', batch_id, 200000 + source_row, record_id, record_ts, 'bt2-' || md5(row_hash), content_hash, layout_id, layout_version,
         txn_utc, time_basis, time_precision, source_tz, account_raw, account_norm, direction, amount + 0.01, currency, balance_after, raw_payload, quality_flags
    FROM forensic.bank_transaction
   WHERE tenant_id = t AND collection_id = c AND evidence_id = ev_a AND NOT ('duplicate_row' = ANY (quality_flags))
   ORDER BY source_row LIMIT 1;
  EXECUTE 'SELECT count(*) FROM analyst.v_bank_transactions' INTO view_after;
  IF view_after <> view_before + 1 THEN RAISE EXCEPTION 'a transaction with another amount was hidden as a repeat (view % -> %)', view_before, view_after; END IF;
  RAISE NOTICE 'a different amount is a different transaction: view % -> %', view_before, view_after;

  -- 4: the same copy inside file A is not touched by the new rule (the writer flags those; here the copy has no flag, so it stays visible)
  INSERT INTO forensic.bank_transaction (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, record_id, record_ts, row_hash, content_hash, layout_id, layout_version,
         txn_utc, time_basis, time_precision, source_tz, account_raw, account_norm, direction, amount, currency, balance_after, raw_payload, quality_flags)
  SELECT tenant_id, collection_id, ev_a, file_id, batch_id, 300000 + source_row, record_id, record_ts, 'bt3-' || md5(row_hash), content_hash, layout_id, layout_version,
         txn_utc, time_basis, time_precision, source_tz, account_raw, account_norm, direction, amount, currency, balance_after, raw_payload, '{}'
    FROM forensic.bank_transaction
   WHERE tenant_id = t AND collection_id = c AND evidence_id = ev_a AND NOT ('duplicate_row' = ANY (quality_flags))
   ORDER BY source_row LIMIT 1;
  EXECUTE 'SELECT count(*) FROM analyst.v_bank_transactions' INTO view_after;
  IF view_after <> view_before + 2 THEN RAISE EXCEPTION 'a repeat inside one file is hidden by the cross-file rule (view % -> %)', view_before, view_after; END IF;
  RAISE NOTICE 'a repeat inside one file is left to the writer''s duplicate_row rule: view % -> %', view_before, view_after;
  RAISE NOTICE 'migration_032_behaviour_ok';
END
$$;

ROLLBACK;
