BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  uncovered text;
BEGIN
  IF EXISTS (
    SELECT 1
    FROM (VALUES ('cdr_frequent_contacts'), ('entity_activity_summary')) AS required(view_name)
    LEFT JOIN pg_class c
      ON c.relnamespace = 'forensic'::regnamespace
     AND c.relname = required.view_name
     AND c.relkind = 'v'
    WHERE c.oid IS NULL
       OR NOT coalesce(c.reloptions, ARRAY[]::text[]) @> ARRAY['security_invoker=true']
  ) THEN
    RAISE EXCEPTION 'forensic query views are not security_invoker';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM (VALUES
      ('records_ingest_jobs', 'tenant_isolation_records_ingest_jobs'),
      ('records_ingest_errors', 'tenant_isolation_records_ingest_errors'),
      ('records_audit_log', 'tenant_isolation_records_audit_log')
    ) AS required(table_name, policy_name)
    LEFT JOIN pg_policies p
      ON p.schemaname = 'forensic'
     AND p.tablename = required.table_name
     AND p.policyname = required.policy_name
     AND p.cmd = 'ALL'
     AND p.permissive = 'PERMISSIVE'
    WHERE p.policyname IS NULL
       OR p.qual IS NULL
       OR p.with_check IS NULL
  ) THEN
    RAISE EXCEPTION 'migration 010 tenant policies are incomplete';
  END IF;

  SELECT string_agg(c.relname, ', ' ORDER BY c.relname)
    INTO uncovered
  FROM pg_class c
  JOIN pg_namespace n ON n.oid = c.relnamespace
  WHERE n.nspname = 'forensic'
    AND c.relkind IN ('r', 'p')
    AND c.relrowsecurity
    AND NOT EXISTS (
      SELECT 1
      FROM pg_policy p
      WHERE p.polrelid = c.oid
    );

  IF uncovered IS NOT NULL THEN
    RAISE EXCEPTION 'RLS-enabled forensic tables without policies: %', uncovered;
  END IF;
END
$$;

ROLLBACK;
