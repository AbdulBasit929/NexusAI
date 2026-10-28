-- Read-only compatibility gate for migration 030 (the five transaction views). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
BEGIN
  IF to_regclass('forensic.bank_transaction') IS NULL THEN
    RAISE EXCEPTION 'migration 029 must be applied before migration 030 (forensic.bank_transaction is missing)';
  END IF;
  IF to_regnamespace('analyst') IS NULL OR to_regclass('analyst.v_comm_events') IS NULL THEN
    RAISE EXCEPTION 'migration 017 must be applied before migration 030 (schema analyst is missing)';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_analyst_ro') THEN
    RAISE EXCEPTION 'the role forensic_analyst_ro is missing (migration 017)';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
             WHERE n.nspname = 'analyst' AND c.relname IN ('v_bank_transactions', 'v_account_counterparties', 'v_account_activity', 'v_account_flows', 'v_account_limit_bands') AND c.relkind <> 'v') THEN
    RAISE EXCEPTION 'a relation named like a new view exists in schema analyst and is not a view';
  END IF;
  RAISE NOTICE 'migration 030 preflight passed';
END
$$;

ROLLBACK;
