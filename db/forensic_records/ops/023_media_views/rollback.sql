-- Rollback for 023_media_views.sql: drops the four views. No table or row was changed by the migration.
-- An Ask engine whose catalogue lists these views must be rolled back with them, or questions that name them are rejected by the validator (the views do not exist); every other class is unaffected.
BEGIN;
DROP VIEW IF EXISTS analyst.v_ocr_evidence_summary;
DROP VIEW IF EXISTS analyst.v_ocr_regions;
DROP VIEW IF EXISTS analyst.v_speech_segments;
DROP VIEW IF EXISTS analyst.v_plate_reads;
COMMIT;
