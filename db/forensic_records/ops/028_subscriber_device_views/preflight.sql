-- Read-only compatibility gate for migration 028 (the views v_subscriber_registrations, v_number_registration_activity and v_handsets). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  missing text;
BEGIN
  IF to_regclass('forensic.subscriber_registration') IS NULL OR to_regclass('forensic.subscriber_identity') IS NULL THEN
    RAISE EXCEPTION 'migration 012 must be applied before migration 028 (forensic.subscriber_registration or subscriber_identity is missing)';
  END IF;
  IF to_regclass('forensic.device_tac') IS NULL THEN
    RAISE EXCEPTION 'migration 013 must be applied before migration 028 (forensic.device_tac is missing)';
  END IF;
  IF to_regnamespace('analyst') IS NULL OR to_regclass('analyst.v_comm_events') IS NULL OR to_regclass('analyst.v_device_pairings') IS NULL THEN
    RAISE EXCEPTION 'migration 017 must be applied before migration 028 (schema analyst, v_comm_events or v_device_pairings is missing)';
  END IF;
  SELECT string_agg(c.name, ', ') INTO missing
  FROM (VALUES ('tenant_id'), ('collection_id'), ('evidence_id'), ('file_id'), ('source_row'), ('registration_id'), ('msisdn'), ('imsi'), ('iccid'), ('subscriber_ref'), ('status'),
               ('operator_label'), ('service_type'), ('valid_from'), ('valid_to'), ('time_basis'), ('source_tz'), ('quality_flags')) AS c(name)
  WHERE NOT EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid = 'forensic.subscriber_registration'::regclass AND a.attname = c.name AND a.attnum > 0 AND NOT a.attisdropped);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION 'forensic.subscriber_registration lacks the columns the registration views read: %', missing;
  END IF;
  SELECT string_agg(c.name, ', ') INTO missing
  FROM (VALUES ('tenant_id'), ('tac'), ('make'), ('model'), ('source')) AS c(name)
  WHERE NOT EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid = 'forensic.device_tac'::regclass AND a.attname = c.name AND a.attnum > 0 AND NOT a.attisdropped);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION 'forensic.device_tac lacks the columns the handset view reads: %', missing;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_analyst_ro') THEN
    RAISE EXCEPTION 'the role forensic_analyst_ro is missing (migration 017)';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
             WHERE n.nspname = 'analyst' AND c.relname IN ('v_subscriber_registrations', 'v_number_registration_activity', 'v_handsets') AND c.relkind <> 'v') THEN
    RAISE EXCEPTION 'a relation named like a new view exists in schema analyst and is not a view';
  END IF;
  -- the identity half must still be dark: nothing may have granted the runtime role a way to read names or national ID numbers
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') AND has_table_privilege('forensic_runtime', 'forensic.subscriber_identity', 'SELECT') THEN
    RAISE EXCEPTION 'forensic_runtime can read forensic.subscriber_identity: the identity half is not dark; do not apply until the masking design is approved';
  END IF;
  RAISE NOTICE 'migration 028 preflight passed';
END
$$;

ROLLBACK;
