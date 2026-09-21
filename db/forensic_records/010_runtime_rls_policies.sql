-- Close the three Phase 1 tenant-policy gaps before non-owner runtime activation.
-- Additive, idempotent and transactional; no retained rows are rewritten.
BEGIN;

ALTER TABLE forensic.records_ingest_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.records_ingest_errors ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.records_audit_log ENABLE ROW LEVEL SECURITY;

-- PostgreSQL views otherwise execute with the view owner's RLS behavior. Force
-- caller permissions so the non-owner runtime role remains subject to base-table
-- tenant policies when the API queries aggregate views.
ALTER VIEW forensic.cdr_frequent_contacts SET (security_invoker = true);
ALTER VIEW forensic.entity_activity_summary SET (security_invoker = true);

DROP POLICY IF EXISTS tenant_isolation_records_ingest_jobs ON forensic.records_ingest_jobs;
CREATE POLICY tenant_isolation_records_ingest_jobs
  ON forensic.records_ingest_jobs
  USING (tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

DROP POLICY IF EXISTS tenant_isolation_records_ingest_errors ON forensic.records_ingest_errors;
CREATE POLICY tenant_isolation_records_ingest_errors
  ON forensic.records_ingest_errors
  USING (tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

DROP POLICY IF EXISTS tenant_isolation_records_audit_log ON forensic.records_audit_log;
CREATE POLICY tenant_isolation_records_audit_log
  ON forensic.records_audit_log
  USING (tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

COMMIT;
