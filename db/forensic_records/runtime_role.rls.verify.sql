-- Synthetic, rolled-back proof that the runtime role is governed by tenant RLS.
-- Run as the schema owner after migration 010 and runtime_role.grants.sql.
BEGIN;
SET LOCAL ROLE forensic_runtime;
SELECT set_config('app.tenant_id', '__nexusai_runtime_rls_probe_a__', true);

DO $$
BEGIN
  INSERT INTO forensic.records_audit_log (
    id, tenant_id, user_id, action, collection_id, details
  ) VALUES (
    -9223372036854775807,
    '__nexusai_runtime_rls_probe_a__',
    'synthetic-runtime-probe',
    'runtime.rls.same_tenant',
    '__nexusai_runtime_rls_probe__',
    '{"synthetic":true,"retained":false}'::jsonb
  );

  IF NOT EXISTS (
    SELECT 1 FROM forensic.records_audit_log
    WHERE id = -9223372036854775807
  ) THEN
    RAISE EXCEPTION 'same-tenant runtime read/write proof failed';
  END IF;

  BEGIN
    INSERT INTO forensic.records_audit_log (
      id, tenant_id, user_id, action, collection_id, details
    ) VALUES (
      -9223372036854775806,
      '__nexusai_runtime_rls_probe_b__',
      'synthetic-runtime-probe',
      'runtime.rls.cross_tenant',
      '__nexusai_runtime_rls_probe__',
      '{"synthetic":true,"retained":false}'::jsonb
    );
    RAISE EXCEPTION 'cross-tenant runtime write unexpectedly succeeded';
  EXCEPTION
    WHEN insufficient_privilege OR check_violation THEN
      NULL;
  END;

  IF EXISTS (
    SELECT 1 FROM forensic.records_audit_log
    WHERE id = -9223372036854775806
  ) THEN
    RAISE EXCEPTION 'cross-tenant runtime row became visible';
  END IF;
END
$$;

ROLLBACK;
