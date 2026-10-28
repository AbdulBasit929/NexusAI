-- Read-only verification of migration 028. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  v text;
BEGIN
  FOREACH v IN ARRAY ARRAY['v_subscriber_registrations', 'v_number_registration_activity', 'v_handsets'] LOOP
    IF to_regclass('analyst.' || v) IS NULL THEN
      RAISE EXCEPTION 'analyst.% is missing (migration 028 is not applied)', v;
    END IF;
    IF obj_description(('analyst.' || v)::regclass, 'pg_class') IS NULL THEN
      RAISE EXCEPTION 'analyst.% has no description', v;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid = ('analyst.' || v)::regclass AND a.attnum > 0 AND NOT a.attisdropped AND col_description(a.attrelid, a.attnum) IS NULL) THEN
      RAISE EXCEPTION 'a column of analyst.% has no description', v;
    END IF;
    IF has_table_privilege('forensic_analyst_ro', ('analyst.' || v)::regclass, 'SELECT') IS NOT TRUE THEN
      RAISE EXCEPTION 'forensic_analyst_ro cannot read analyst.%', v;
    END IF;
    IF has_table_privilege('public', ('analyst.' || v)::regclass, 'SELECT') THEN
      RAISE EXCEPTION 'PUBLIC can read analyst.%', v;
    END IF;
    IF pg_get_viewdef(('analyst.' || v)::regclass) ILIKE '%subscriber_identity%' THEN
      RAISE EXCEPTION 'analyst.% reads forensic.subscriber_identity: identity must stay out of every view', v;
    END IF;
  END LOOP;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_subscriber_registrations'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'registration_id,evidence_id,source_file,source_row,msisdn,imsi,iccid,subscriber_ref,status,operator_label,service_type,valid_from,valid_to,time_basis,source_tz,conflicts' THEN
    RAISE EXCEPTION 'analyst.v_subscriber_registrations has other columns than the migration gave it';
  END IF;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_number_registration_activity'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'msisdn,registrations,sources,first_valid_from,last_valid_to,conflicts,events,events_outside_registration,events_after_last_end,first_event_utc,last_event_utc' THEN
    RAISE EXCEPTION 'analyst.v_number_registration_activity has other columns than the migration gave it';
  END IF;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_handsets'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'src,imei,imsi,events,first_seen,last_seen,tac,make,model,listed_by,tac_listed' THEN
    RAISE EXCEPTION 'analyst.v_handsets has other columns than the migration gave it';
  END IF;
  IF pg_get_viewdef('analyst.v_subscriber_registrations'::regclass) NOT LIKE '%app.tenant_id%' OR pg_get_viewdef('analyst.v_subscriber_registrations'::regclass) NOT LIKE '%app.collection_id%' THEN
    RAISE EXCEPTION 'analyst.v_subscriber_registrations no longer carries its own tenant and collection scope';
  END IF;
  IF pg_get_viewdef('analyst.v_number_registration_activity'::regclass) NOT LIKE '%v_subscriber_registrations%' OR pg_get_viewdef('analyst.v_number_registration_activity'::regclass) NOT LIKE '%v_comm_events%' THEN
    RAISE EXCEPTION 'analyst.v_number_registration_activity does not read the scoped views (it takes its scope from them)';
  END IF;
  IF pg_get_viewdef('analyst.v_handsets'::regclass) NOT LIKE '%v_device_pairings%' OR pg_get_viewdef('analyst.v_handsets'::regclass) NOT LIKE '%app.tenant_id%' THEN
    RAISE EXCEPTION 'analyst.v_handsets no longer takes its case scope from v_device_pairings or the tenant scope of the device reference';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') AND has_table_privilege('forensic_runtime', 'forensic.subscriber_identity', 'SELECT') THEN
    RAISE EXCEPTION 'forensic_runtime can read forensic.subscriber_identity';
  END IF;
  IF has_table_privilege('forensic_analyst_ro', 'forensic.subscriber_identity', 'SELECT') OR has_table_privilege('forensic_analyst_ro', 'forensic.subscriber_registration', 'SELECT') THEN
    RAISE EXCEPTION 'forensic_analyst_ro can read a subscriber table directly: it may read only through the views';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'forensic' AND table_name = 'subscriber_registration' AND column_name = 'time_basis' AND is_nullable = 'YES') THEN
    RAISE EXCEPTION 'forensic.subscriber_registration.time_basis still refuses an empty value (migration 028 relaxes it)';
  END IF;
  RAISE NOTICE 'migration_028_verified';
END
$$;

ROLLBACK;
