-- Read-only verification of migration 024. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  v text;
BEGIN
  FOREACH v IN ARRAY ARRAY['v_colocation_windows', 'v_meeting_candidates'] LOOP
    IF to_regclass('analyst.' || v) IS NULL THEN
      RAISE EXCEPTION 'analyst.% is missing (migration 024 is not applied)', v;
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
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_colocation_windows'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'a,b,cell_key,start_utc,end_utc,pairs,window_minutes,site_name,lat,lon,radius_m' THEN
    RAISE EXCEPTION 'analyst.v_colocation_windows has other columns than the migration gave it';
  END IF;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_meeting_candidates'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'a,b,cell_key,start_utc,end_utc,pairs,contacts,last_contact_utc,gap_minutes,score,window_minutes,allowed_gap_minutes,site_name,lat,lon,radius_m' THEN
    RAISE EXCEPTION 'analyst.v_meeting_candidates has other columns than the migration gave it';
  END IF;
  IF pg_get_viewdef('analyst.v_colocation_windows'::regclass) NOT LIKE '%v_comm_events%' THEN
    RAISE EXCEPTION 'analyst.v_colocation_windows does not read analyst.v_comm_events (it takes its tenant and collection scope from it)';
  END IF;
  IF pg_get_viewdef('analyst.v_colocation_windows'::regclass) NOT LIKE '%app.colocation_minutes%' THEN
    RAISE EXCEPTION 'analyst.v_colocation_windows no longer reads the window from app.colocation_minutes (it must be a runtime parameter, not a constant)';
  END IF;
  IF pg_get_viewdef('analyst.v_meeting_candidates'::regclass) NOT LIKE '%v_colocation_windows%' OR pg_get_viewdef('analyst.v_meeting_candidates'::regclass) NOT LIKE '%app.contact_gap_minutes%' THEN
    RAISE EXCEPTION 'analyst.v_meeting_candidates does not read the co-location windows or no longer reads the gap from app.contact_gap_minutes';
  END IF;
  RAISE NOTICE 'migration_024_verified';
END
$$;

ROLLBACK;
