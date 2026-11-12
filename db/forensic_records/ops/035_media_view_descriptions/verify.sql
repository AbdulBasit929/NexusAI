-- Read-only verification of migration 035. Any exception means the descriptions are not the ones of the migration.
BEGIN TRANSACTION READ ONLY;
DO $$
BEGIN
  IF col_description('analyst.v_plate_reads'::regclass, (SELECT attnum FROM pg_attribute WHERE attrelid = 'analyst.v_plate_reads'::regclass AND attname = 'first_seen_seconds')) NOT LIKE '%last_seen_seconds >= A%' THEN
    RAISE EXCEPTION 'v_plate_reads.first_seen_seconds does not carry the window description (migration 035 is not applied)';
  END IF;
  IF col_description('analyst.v_video_plate_frames'::regclass, (SELECT attnum FROM pg_attribute WHERE attrelid = 'analyst.v_video_plate_frames'::regclass AND attname = 'seen_at_seconds')) NOT LIKE '%BETWEEN A AND B%' THEN
    RAISE EXCEPTION 'v_video_plate_frames.seen_at_seconds does not carry the window description';
  END IF;
  IF col_description('analyst.v_plate_reads'::regclass, (SELECT attnum FROM pg_attribute WHERE attrelid = 'analyst.v_plate_reads'::regclass AND attname = 'source')) NOT LIKE '%group by original_filename%' THEN
    RAISE EXCEPTION 'v_plate_reads.source does not carry the grouping description';
  END IF;
  IF col_description('analyst.v_ocr_regions'::regclass, (SELECT attnum FROM pg_attribute WHERE attrelid = 'analyst.v_ocr_regions'::regclass AND attname = 'needs_review')) NOT LIKE '%needs_review = false matches no row%' THEN
    RAISE EXCEPTION 'v_ocr_regions.needs_review does not carry the description of migration 035';
  END IF;
  RAISE NOTICE 'migration_035_verified';
END
$$;
ROLLBACK;
