-- Rollback for 026_movement_and_rhythm.sql: drops the seven views (dependents first). No table or row was changed by the migration.
-- An Ask engine whose catalogue lists these views (the movement, speed and rhythm examples) must be rolled back with them, or questions that name them are rejected by the validator
-- (the views do not exist); questions of every other class are unaffected.
BEGIN;
DROP VIEW IF EXISTS analyst.v_rhythm_changes;
DROP VIEW IF EXISTS analyst.v_speed_alerts;
DROP VIEW IF EXISTS analyst.v_anchor_deviations;
DROP VIEW IF EXISTS analyst.v_anchor_changes;
DROP VIEW IF EXISTS analyst.v_movement_anchors;
DROP VIEW IF EXISTS analyst.v_anchor_runs;
DROP VIEW IF EXISTS analyst.v_weekly_anchors;
COMMIT;
