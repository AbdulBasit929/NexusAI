-- Read-only verification of migration 025. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  v text;
BEGIN
  FOREACH v IN ARRAY ARRAY['v_silent_numbers', 'v_successor_candidates'] LOOP
    IF to_regclass('analyst.' || v) IS NULL THEN
      RAISE EXCEPTION 'analyst.% is missing (migration 025 is not applied)', v;
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
    IF pg_get_viewdef(('analyst.' || v)::regclass) NOT LIKE '%v_comm_events%' THEN
      RAISE EXCEPTION 'analyst.% does not read analyst.v_comm_events (it takes its tenant and collection scope from it)', v;
    END IF;
  END LOOP;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_silent_numbers'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'number,first_event_utc,last_event_utc,events,days_silent,silence_days' THEN
    RAISE EXCEPTION 'analyst.v_silent_numbers has other columns than the migration gave it';
  END IF;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_successor_candidates'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'old_number,new_number,old_last_event_utc,new_first_event_utc,score,contact_cosine,shared_contacts,cell_jaccard,shared_device,silence_days,window_days,score_percent' THEN
    RAISE EXCEPTION 'analyst.v_successor_candidates has other columns than the migration gave it';
  END IF;
  IF pg_get_viewdef('analyst.v_successor_candidates'::regclass) NOT LIKE '%app.silence_days%' OR pg_get_viewdef('analyst.v_successor_candidates'::regclass) NOT LIKE '%app.successor_window_days%'
     OR pg_get_viewdef('analyst.v_successor_candidates'::regclass) NOT LIKE '%app.successor_score_percent%' THEN
    RAISE EXCEPTION 'analyst.v_successor_candidates no longer reads its rules from the transaction settings (they must be runtime parameters, not constants)';
  END IF;
  IF pg_get_viewdef('analyst.v_silent_numbers'::regclass) NOT LIKE '%app.silence_days%' THEN
    RAISE EXCEPTION 'analyst.v_silent_numbers no longer reads the silence from app.silence_days';
  END IF;
  RAISE NOTICE 'migration_025_verified';
END
$$;

ROLLBACK;
