-- Read-only verification of migration 027. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  v text;
BEGIN
  FOREACH v IN ARRAY ARRAY['v_plate_sightings', 'v_plate_visits'] LOOP
    IF to_regclass('analyst.' || v) IS NULL THEN
      RAISE EXCEPTION 'analyst.% is missing (migration 027 is not applied)', v;
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
  END LOOP;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_plate_sightings'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'sighting_id,evidence_id,source_row,plate_norm,plate_raw,observed_utc,camera_id,camera_label,lane,lat,lon,loc_method,confidence,time_basis,source_tz' THEN
    RAISE EXCEPTION 'analyst.v_plate_sightings has other columns than the migration gave it';
  END IF;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_plate_visits'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'plate_norm,camera_id,camera_label,lat,lon,sightings,first_utc,last_utc' THEN
    RAISE EXCEPTION 'analyst.v_plate_visits has other columns than the migration gave it';
  END IF;
  IF pg_get_viewdef('analyst.v_plate_sightings'::regclass) NOT LIKE '%app.tenant_id%' OR pg_get_viewdef('analyst.v_plate_sightings'::regclass) NOT LIKE '%app.collection_id%' THEN
    RAISE EXCEPTION 'analyst.v_plate_sightings no longer carries its own tenant and collection scope';
  END IF;
  IF pg_get_viewdef('analyst.v_plate_sightings'::regclass) NOT LIKE '%source_kind%csv%' THEN
    RAISE EXCEPTION 'analyst.v_plate_sightings no longer leaves out plates read from images and video';
  END IF;
  IF pg_get_viewdef('analyst.v_plate_visits'::regclass) NOT LIKE '%v_plate_sightings%' THEN
    RAISE EXCEPTION 'analyst.v_plate_visits does not read analyst.v_plate_sightings (it takes its scope from it)';
  END IF;
  RAISE NOTICE 'migration_027_verified';
END
$$;

ROLLBACK;
