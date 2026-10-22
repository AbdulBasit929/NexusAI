-- Rollback for 024_colocation_and_meetings.sql: drops the two views. No table or row was changed by the migration.
-- An Ask engine whose catalogue lists these views (examples co-location-15 and meeting-candidates) must be rolled back with them, or questions that name them are rejected by the validator
-- (the views do not exist); questions of every other class are unaffected.
BEGIN;
DROP VIEW IF EXISTS analyst.v_meeting_candidates;
DROP VIEW IF EXISTS analyst.v_colocation_windows;
COMMIT;
