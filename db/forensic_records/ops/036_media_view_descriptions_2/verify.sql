-- Read-only verification of migration 036. Any exception means the descriptions are not the ones of the migration.
BEGIN TRANSACTION READ ONLY;
DO $$
BEGIN
  IF obj_description('analyst.v_video_files'::regclass, 'pg_class') NOT LIKE '%and not v_evidence_inventory%' THEN
    RAISE EXCEPTION 'v_video_files does not carry the description of migration 036';
  END IF;
  IF col_description('analyst.v_video_plate_frames'::regclass, (SELECT attnum FROM pg_attribute WHERE attrelid = 'analyst.v_video_plate_frames'::regclass AND attname = 'frame_number')) NOT LIKE '%count the reads count the rows%' THEN
    RAISE EXCEPTION 'v_video_plate_frames.frame_number does not carry the description of migration 036';
  END IF;
  IF col_description('analyst.v_video_files'::regclass, (SELECT attnum FROM pg_attribute WHERE attrelid = 'analyst.v_video_files'::regclass AND attname = 'has_audio')) NOT LIKE '%use v_speech_segments%' THEN
    RAISE EXCEPTION 'v_video_files.has_audio does not carry the description of migration 036';
  END IF;
  RAISE NOTICE 'migration_036_verified';
END
$$;
ROLLBACK;
