-- Rollback for 023_media_views.sql: drops the six views. No table or row was changed by the migration.
-- An Ask engine whose catalogue lists these views must be rolled back with them, or questions that name them are rejected by the validator (the views do not exist); every other class is unaffected.
BEGIN;
DROP VIEW IF EXISTS analyst.v_ocr_evidence_summary;
DROP VIEW IF EXISTS analyst.v_ocr_regions;
DROP VIEW IF EXISTS analyst.v_speech_segments;
DROP VIEW IF EXISTS analyst.v_plate_reads;
DROP VIEW IF EXISTS analyst.v_video_plate_frames;
DROP VIEW IF EXISTS analyst.v_video_files;
COMMIT;
