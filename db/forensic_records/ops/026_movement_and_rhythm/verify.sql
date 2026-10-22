-- Read-only verification of migration 026. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  v text;
BEGIN
  FOREACH v IN ARRAY ARRAY['v_weekly_anchors', 'v_anchor_runs', 'v_movement_anchors', 'v_anchor_changes', 'v_anchor_deviations', 'v_speed_alerts', 'v_rhythm_changes'] LOOP
    IF to_regclass('analyst.' || v) IS NULL THEN
      RAISE EXCEPTION 'analyst.% is missing (migration 026 is not applied)', v;
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
  IF pg_get_viewdef('analyst.v_weekly_anchors'::regclass) NOT LIKE '%v_comm_events%' OR pg_get_viewdef('analyst.v_weekly_anchors'::regclass) NOT LIKE '%app.anchor_min_events%' THEN
    RAISE EXCEPTION 'analyst.v_weekly_anchors does not read analyst.v_comm_events or no longer reads app.anchor_min_events';
  END IF;
  IF pg_get_viewdef('analyst.v_anchor_runs'::regclass) NOT LIKE '%app.anchor_hold_weeks%' THEN
    RAISE EXCEPTION 'analyst.v_anchor_runs no longer reads app.anchor_hold_weeks (it must be a runtime parameter, not a constant)';
  END IF;
  IF pg_get_viewdef('analyst.v_speed_alerts'::regclass) NOT LIKE '%app.speed_kmh%' THEN
    RAISE EXCEPTION 'analyst.v_speed_alerts no longer reads app.speed_kmh';
  END IF;
  IF pg_get_viewdef('analyst.v_rhythm_changes'::regclass) NOT LIKE '%app.rhythm_min_side_days%' OR pg_get_viewdef('analyst.v_rhythm_changes'::regclass) NOT LIKE '%app.rhythm_ratio_percent%' THEN
    RAISE EXCEPTION 'analyst.v_rhythm_changes no longer reads its thresholds from the transaction settings';
  END IF;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_anchor_changes'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'number,slot,from_cell_key,to_cell_key,first_week,first_week_start,distance_km,hold_weeks' THEN
    RAISE EXCEPTION 'analyst.v_anchor_changes has other columns than the migration gave it';
  END IF;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_rhythm_changes'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'number,days,first_day,mean_before,mean_after,t,ratio,direction,min_side_days,ratio_percent' THEN
    RAISE EXCEPTION 'analyst.v_rhythm_changes has other columns than the migration gave it';
  END IF;
  RAISE NOTICE 'migration_026_verified';
END
$$;

ROLLBACK;
