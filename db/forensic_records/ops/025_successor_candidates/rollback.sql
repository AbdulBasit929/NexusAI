-- Rollback for 025_successor_candidates.sql: drops the two views. No table or row was changed by the migration.
-- An Ask engine whose catalogue lists these views (examples silent-numbers and successor-candidates) must be rolled back with them, or questions that name them are rejected by the validator
-- (the views do not exist); questions of every other class are unaffected.
BEGIN;
DROP VIEW IF EXISTS analyst.v_successor_candidates;
DROP VIEW IF EXISTS analyst.v_silent_numbers;
COMMIT;
