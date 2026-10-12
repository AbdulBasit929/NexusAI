-- Read-only verification of migration 013. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  problem text;
BEGIN
  SELECT string_agg(t.name, ', ')
    INTO problem
  FROM (VALUES ('cell_site'), ('camera_site'), ('device_tac'), ('ip_range_reference')) AS t(name)
  WHERE to_regclass('forensic.' || t.name) IS NULL;
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'missing tables: %', problem;
  END IF;

  SELECT string_agg(t.name, ', ')
    INTO problem
  FROM (VALUES ('cell_site'), ('camera_site'), ('device_tac'), ('ip_range_reference')) AS t(name)
  WHERE NOT EXISTS (SELECT 1 FROM pg_class c WHERE c.oid = ('forensic.' || t.name)::regclass AND c.relrowsecurity)
     OR NOT EXISTS (
       SELECT 1 FROM pg_policies p
       WHERE p.schemaname = 'forensic' AND p.tablename = t.name
         AND p.policyname = 'tenant_isolation_' || t.name
         AND p.cmd = 'ALL' AND p.permissive = 'PERMISSIVE'
         AND p.qual IS NOT NULL AND p.with_check IS NOT NULL
     );
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'row-level security or tenant policy missing on: %', problem;
  END IF;

  SELECT string_agg(c.rel || '.' || c.conname, ', ')
    INTO problem
  FROM (VALUES
    ('cell_site', 'cell_site_key_chk'), ('cell_site', 'cell_site_coord_chk'), ('cell_site', 'cell_site_geometry_chk'),
    ('camera_site', 'camera_site_coord_chk'), ('device_tac', 'device_tac_tac_chk'),
    ('ip_range_reference', 'ip_range_reference_country_chk'), ('ip_range_reference', 'ip_range_reference_asn_chk')
  ) AS c(rel, conname)
  WHERE NOT EXISTS (
    SELECT 1 FROM pg_constraint k
    WHERE k.conrelid = ('forensic.' || c.rel)::regclass AND k.conname = c.conname AND k.convalidated
  );
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'missing or unvalidated constraints: %', problem;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_indexes
    WHERE schemaname = 'forensic' AND tablename = 'ip_range_reference'
      AND indexname = 'ip_range_reference_network_idx' AND indexdef LIKE '%USING gist%inet_ops%'
  ) THEN
    RAISE EXCEPTION 'the GiST inet_ops index on ip_range_reference is missing';
  END IF;

  -- Cell keys: pure function checks.
  IF forensic.make_cell_key('41001', '0100', '000200') IS DISTINCT FROM '41001:100:200'
     OR forensic.make_cell_key(NULL, '100', '200') IS DISTINCT FROM '?:100:200'
     OR forensic.make_cell_key('41001', NULL, '200') IS DISTINCT FROM '41001:?:200'
     OR forensic.make_cell_key('41001', '100', NULL) IS NOT NULL
     OR forensic.make_cell_key('41001', '100', '   ') IS NOT NULL
     OR forensic.make_cell_key(' 41001 ', ' 100 ', ' 0 ') IS DISTINCT FROM '41001:100:0'
     OR forensic.make_cell_key('41001', 'AB1', 'Cd2') IS DISTINCT FROM '41001:ab1:cd2'
     OR forensic.make_cell_key('00101', '100', '200') IS DISTINCT FROM '00101:100:200' THEN
    RAISE EXCEPTION 'forensic.make_cell_key does not behave as specified';
  END IF;

  IF EXISTS (SELECT 1 FROM forensic.cell_site c
             JOIN forensic.evidence_items e ON e.evidence_id = c.evidence_id AND e.tenant_id <> c.tenant_id) THEN
    RAISE EXCEPTION 'cell_site rows point at evidence of another tenant';
  END IF;
  IF EXISTS (SELECT 1 FROM forensic.camera_site c
             JOIN forensic.evidence_items e ON e.evidence_id = c.evidence_id AND e.tenant_id <> c.tenant_id) THEN
    RAISE EXCEPTION 'camera_site rows point at evidence of another tenant';
  END IF;

  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    SELECT string_agg(t.name, ', ')
      INTO problem
    FROM (VALUES ('cell_site'), ('camera_site'), ('device_tac'), ('ip_range_reference')) AS t(name)
    WHERE NOT has_table_privilege('forensic_runtime', 'forensic.' || t.name, 'SELECT')
       OR NOT has_table_privilege('forensic_runtime', 'forensic.' || t.name, 'INSERT')
       OR has_table_privilege('forensic_runtime', 'forensic.' || t.name, 'UPDATE')
       OR has_table_privilege('forensic_runtime', 'forensic.' || t.name, 'DELETE')
       OR has_table_privilege('forensic_runtime', 'forensic.' || t.name, 'TRUNCATE');
    IF problem IS NOT NULL THEN
      RAISE EXCEPTION 'runtime role privileges are wrong on: %', problem;
    END IF;
  END IF;
END
$$;

SELECT 'migration_013_verified' AS result;

ROLLBACK;
