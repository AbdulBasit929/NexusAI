-- Read-only verification of migration 012. Run it right after 012 (and again after any restore).
-- Any exception means the migration is incomplete or was altered: do not start writers; consider rollback.sql.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  problem text;
BEGIN
  -- 1. Tables exist; the event tables are Timescale hypertables with a seven-day chunk interval.
  SELECT string_agg(t.name, ', ')
    INTO problem
  FROM (VALUES ('layout_versions'), ('comm_event'), ('data_session'), ('app_usage'), ('plate_sighting'),
               ('subscriber_identity'), ('subscriber_registration')) AS t(name)
  WHERE to_regclass('forensic.' || t.name) IS NULL;
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'missing tables: %', problem;
  END IF;

  SELECT string_agg(t.name, ', ')
    INTO problem
  FROM (VALUES ('comm_event', 'start_utc'), ('data_session', 'start_utc'), ('app_usage', 'start_utc'),
               ('plate_sighting', 'observed_utc')) AS t(name, time_column)
  WHERE NOT EXISTS (
    SELECT 1
    FROM timescaledb_information.dimensions d
    WHERE d.hypertable_schema = 'forensic'
      AND d.hypertable_name = t.name
      AND d.column_name = t.time_column
      AND d.time_interval = INTERVAL '7 days'
  );
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'tables that are not hypertables chunked by seven days on the expected column: %', problem;
  END IF;

  -- 2. Row-level security is on, and every tenant table has the tenant policy with both USING and WITH CHECK.
  SELECT string_agg(t.name, ', ')
    INTO problem
  FROM (VALUES ('comm_event'), ('data_session'), ('app_usage'), ('plate_sighting'),
               ('subscriber_identity'), ('subscriber_registration')) AS t(name)
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

  IF EXISTS (SELECT 1 FROM pg_class WHERE oid = 'forensic.layout_versions'::regclass AND relrowsecurity) THEN
    RAISE EXCEPTION 'layout_versions is global reference data and must not have row-level security';
  END IF;

  -- 3. Lineage and integrity constraints are present by name.
  SELECT string_agg(c.rel || '.' || c.conname, ', ')
    INTO problem
  FROM (VALUES
    ('comm_event', 'comm_event_layout_fk'), ('comm_event', 'comm_event_evidence_fk'), ('comm_event', 'comm_event_payload_chk'),
    ('comm_event', 'comm_event_direction_chk'), ('comm_event', 'comm_event_time_basis_chk'), ('comm_event', 'comm_event_coord_chk'),
    ('data_session', 'data_session_layout_fk'), ('data_session', 'data_session_evidence_fk'), ('data_session', 'data_session_ports_chk'),
    ('app_usage', 'app_usage_layout_fk'), ('app_usage', 'app_usage_evidence_fk'),
    ('plate_sighting', 'plate_sighting_layout_fk'), ('plate_sighting', 'plate_sighting_evidence_fk'),
    ('plate_sighting', 'plate_sighting_artifact_fk'), ('plate_sighting', 'plate_sighting_confidence_chk'),
    ('subscriber_registration', 'subscriber_registration_evidence_fk'), ('subscriber_registration', 'subscriber_registration_identity_fk'),
    ('subscriber_identity', 'subscriber_identity_evidence_fk'),
    ('layout_versions', 'layout_versions_sha_chk')
  ) AS c(rel, conname)
  WHERE NOT EXISTS (
    SELECT 1 FROM pg_constraint k
    WHERE k.conrelid = ('forensic.' || c.rel)::regclass AND k.conname = c.conname AND k.convalidated
  );
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'missing or unvalidated constraints: %', problem;
  END IF;

  -- 4. Idempotency keys exist on the event tables (re-loading a file must not duplicate rows).
  SELECT string_agg(t.name, ', ')
    INTO problem
  FROM (VALUES ('comm_event'), ('data_session'), ('app_usage'), ('plate_sighting')) AS t(name)
  WHERE NOT EXISTS (
    SELECT 1 FROM pg_constraint k
    WHERE k.conrelid = ('forensic.' || t.name)::regclass AND k.contype = 'u'
      AND (SELECT array_agg(a.attname::text ORDER BY a.attname::text) FROM pg_attribute a
           WHERE a.attrelid = k.conrelid AND a.attnum = ANY (k.conkey))
          @> ARRAY['collection_id', 'file_id', 'row_hash', 'tenant_id']
  );
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'idempotency unique key missing on: %', problem;
  END IF;

  -- 5. The fallback layout row exists.
  IF NOT EXISTS (SELECT 1 FROM forensic.layout_versions WHERE layout_id = 'generic-unmapped' AND layout_version = 1) THEN
    RAISE EXCEPTION 'fallback layout generic-unmapped v1 is missing';
  END IF;

  -- 6. The plate search key behaves as documented (pure function checks, no data read).
  IF forensic.normalize_plate(' lea-1234 ') IS DISTINCT FROM 'LEA1234'
     OR forensic.normalize_plate('ABC 123') IS DISTINCT FROM 'ABC123'
     OR forensic.normalize_plate(U&'\0661\0662\0663') IS DISTINCT FROM '123'
     OR forensic.normalize_plate(U&'\06F4\06F5\06F6-AB') IS DISTINCT FROM '456AB'
     OR forensic.normalize_plate(U&'\FF21\FF22\FF11') IS DISTINCT FROM 'AB1'
     OR forensic.normalize_plate(NULL) IS NOT NULL THEN
    RAISE EXCEPTION 'forensic.normalize_plate does not behave as specified';
  END IF;

  -- 7. Runtime role privileges, when the role exists: read and append on the event tables, no update or delete,
  --    and no privilege at all on the identity table.
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    SELECT string_agg(t.name, ', ')
      INTO problem
    FROM (VALUES ('comm_event'), ('data_session'), ('app_usage'), ('plate_sighting'), ('subscriber_registration')) AS t(name)
    WHERE NOT has_table_privilege('forensic_runtime', 'forensic.' || t.name, 'SELECT')
       OR NOT has_table_privilege('forensic_runtime', 'forensic.' || t.name, 'INSERT')
       OR has_table_privilege('forensic_runtime', 'forensic.' || t.name, 'UPDATE')
       OR has_table_privilege('forensic_runtime', 'forensic.' || t.name, 'DELETE')
       OR has_table_privilege('forensic_runtime', 'forensic.' || t.name, 'TRUNCATE');
    IF problem IS NOT NULL THEN
      RAISE EXCEPTION 'runtime role privileges are wrong on: %', problem;
    END IF;

    IF has_table_privilege('forensic_runtime', 'forensic.subscriber_identity', 'SELECT')
       OR has_table_privilege('forensic_runtime', 'forensic.subscriber_identity', 'INSERT')
       OR has_table_privilege('forensic_runtime', 'forensic.subscriber_identity', 'UPDATE') THEN
      RAISE EXCEPTION 'forensic_runtime must have no privilege on forensic.subscriber_identity';
    END IF;
  END IF;
END
$$;

SELECT 'migration_012_verified' AS result;

ROLLBACK;
