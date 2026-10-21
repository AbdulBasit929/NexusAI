-- Read-only verification of migration 017. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  problem text;
  expected text[] := ARRAY['v_app_usage', 'v_cell_visits', 'v_cells', 'v_comm_events', 'v_connection_edges', 'v_contact_links', 'v_contact_pairs', 'v_data_sessions', 'v_device_pairings', 'v_evidence_inventory']; -- the last three views are migration 020's, reviewed there
  found text[];
BEGIN
  SELECT array_agg(c.relname::text ORDER BY c.relname) INTO found
  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
  WHERE n.nspname = 'analyst' AND c.relkind IN ('v', 'r', 'm', 'f', 'p');
  IF found IS DISTINCT FROM expected THEN
    RAISE EXCEPTION 'schema analyst holds % but exactly % were expected (an unreviewed relation must not be there)', found, expected;
  END IF;
  IF EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = 'analyst' AND c.relkind <> 'v') THEN
    RAISE EXCEPTION 'schema analyst must hold plain views only';
  END IF;

  -- Scope is structural: the views read as their owner, so each must carry its own tenant predicate. A security_invoker view would not need it, and one
  -- that has lost it would leak across tenants; both the option and the text are checked.
  SELECT string_agg(c.relname, ', ') INTO problem
  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
  WHERE n.nspname = 'analyst' AND c.relname IN ('v_comm_events', 'v_cells', 'v_data_sessions', 'v_app_usage')
    AND pg_get_viewdef(c.oid) NOT LIKE '%app.tenant_id%';
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'these views do not filter on app.tenant_id: %', problem;
  END IF;
  SELECT string_agg(c.relname, ', ') INTO problem
  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
  WHERE n.nspname = 'analyst' AND c.relname IN ('v_comm_events', 'v_cells', 'v_data_sessions', 'v_app_usage')
    AND pg_get_viewdef(c.oid) NOT LIKE '%app.collection_id%';
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'these views do not filter on app.collection_id: %', problem;
  END IF;

  -- Every view and every column is described: the schema catalogue is generated from the descriptions.
  SELECT string_agg(c.relname, ', ') INTO problem
  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
  WHERE n.nspname = 'analyst' AND obj_description(c.oid, 'pg_class') IS NULL;
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'views without a description: %', problem;
  END IF;
  SELECT string_agg(c.relname || '.' || a.attname, ', ') INTO problem
  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
  JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
  WHERE n.nspname = 'analyst' AND col_description(c.oid, a.attnum) IS NULL;
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'columns without a description: %', problem;
  END IF;

  -- The read-only role.
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_analyst_ro') THEN
    RAISE EXCEPTION 'role forensic_analyst_ro is missing';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_analyst_ro' AND (rolcanlogin OR rolsuper OR rolbypassrls OR rolcreatedb OR rolcreaterole)) THEN
    RAISE EXCEPTION 'forensic_analyst_ro must be NOLOGIN, not superuser, no BYPASSRLS, no CREATEDB, no CREATEROLE';
  END IF;
  IF NOT has_schema_privilege('forensic_analyst_ro', 'analyst', 'USAGE') THEN
    RAISE EXCEPTION 'forensic_analyst_ro cannot use schema analyst';
  END IF;
  SELECT string_agg(c.relname, ', ') INTO problem
  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
  WHERE n.nspname = 'analyst' AND NOT has_table_privilege('forensic_analyst_ro', c.oid, 'SELECT');
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'forensic_analyst_ro cannot read: %', problem;
  END IF;
  SELECT string_agg(c.relname, ', ') INTO problem
  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
  WHERE n.nspname = 'analyst'
    AND (has_table_privilege('forensic_analyst_ro', c.oid, 'INSERT') OR has_table_privilege('forensic_analyst_ro', c.oid, 'UPDATE')
         OR has_table_privilege('forensic_analyst_ro', c.oid, 'DELETE') OR has_table_privilege('forensic_analyst_ro', c.oid, 'TRUNCATE'));
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'forensic_analyst_ro can write to: %', problem;
  END IF;

  -- The point of the role: no base table and no other schema of the application.
  SELECT string_agg(n.nspname || '.' || c.relname, ', ') INTO problem
  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
  WHERE n.nspname NOT IN ('analyst', 'pg_catalog', 'information_schema', 'pg_toast') AND c.relkind IN ('r', 'v', 'm', 'p', 'f')
    -- TimescaleDB's own metadata views are readable by every role by default and hold no case data; its CHUNKS hold case rows and are not excused.
    AND n.nspname NOT IN ('timescaledb_information', 'timescaledb_experimental', '_timescaledb_config', '_timescaledb_catalog', '_timescaledb_cache', '_timescaledb_functions')
    AND NOT (n.nspname = '_timescaledb_internal' AND c.relname !~ '^(_hyper_|compress_hyper_|_materialized_hypertable_|_direct_view_|_partial_view_)')
    AND (has_table_privilege('forensic_analyst_ro', c.oid, 'SELECT') OR has_table_privilege('forensic_analyst_ro', c.oid, 'INSERT')
         OR has_table_privilege('forensic_analyst_ro', c.oid, 'UPDATE') OR has_table_privilege('forensic_analyst_ro', c.oid, 'DELETE'));
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'forensic_analyst_ro can reach relations outside the analyst schema: %', problem;
  END IF;
  IF has_schema_privilege('forensic_analyst_ro', 'forensic', 'USAGE') THEN
    RAISE EXCEPTION 'forensic_analyst_ro must not have USAGE on schema forensic';
  END IF;

  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime')
     AND NOT EXISTS (SELECT 1 FROM pg_auth_members m JOIN pg_roles r ON r.oid = m.roleid JOIN pg_roles u ON u.oid = m.member
                     WHERE r.rolname = 'forensic_analyst_ro' AND u.rolname = 'forensic_runtime') THEN
    RAISE EXCEPTION 'forensic_runtime is not a member of forensic_analyst_ro (it could not SET ROLE)';
  END IF;

  RAISE NOTICE 'migration_017_verified';
END
$$;

ROLLBACK;
