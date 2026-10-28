-- Rollback for 030_transaction_views.sql: drops the five views. The table of migration 029 and its rows are not touched.
-- An Ask engine whose catalogue lists these views (and the transaction examples) must be rolled back with them, or questions that name them are rejected by the validator (the views do not exist);
-- questions of every other class are unaffected.
BEGIN;
DROP VIEW IF EXISTS analyst.v_account_limit_bands;
DROP VIEW IF EXISTS analyst.v_account_flows;
DROP VIEW IF EXISTS analyst.v_account_activity;
DROP VIEW IF EXISTS analyst.v_account_counterparties;
DROP VIEW IF EXISTS analyst.v_bank_transactions;
COMMIT;
