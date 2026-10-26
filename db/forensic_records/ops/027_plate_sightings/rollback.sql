-- Rollback for 027_plate_sightings.sql: drops the two views. No table or row was changed by the migration.
-- An Ask engine whose catalogue lists these views (and the plate examples) must be rolled back with them, or questions that name them are rejected by the validator
-- (the views do not exist); questions of every other class are unaffected.
BEGIN;
DROP VIEW IF EXISTS analyst.v_plate_visits;
DROP VIEW IF EXISTS analyst.v_plate_sightings;
COMMIT;
