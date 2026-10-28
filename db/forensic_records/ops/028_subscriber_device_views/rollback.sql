-- Rollback for 028_subscriber_device_views.sql: drops the three views, and restores the old rule that a registration row has a time basis.
-- An Ask engine whose catalogue lists these views (and the registration and handset examples) must be rolled back with them, or questions that name them are rejected by the validator
-- (the views do not exist); questions of every other class are unaffected.
-- The time_basis rule is restored only while no row without one exists: a row written without a date after the migration has no basis to give it, and none is invented. Then the whole
-- script is one transaction that stops with a message and changes nothing (the views stay), and the rows are reviewed by hand (the table is empty until the typed writer for registrations is deployed).
BEGIN;
DROP VIEW IF EXISTS analyst.v_number_registration_activity;
DROP VIEW IF EXISTS analyst.v_subscriber_registrations;
DROP VIEW IF EXISTS analyst.v_handsets;
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM forensic.subscriber_registration WHERE time_basis IS NULL) THEN
    RAISE EXCEPTION 'rows without a time basis exist in forensic.subscriber_registration: the views were NOT dropped (this transaction is rolled back); review those rows first';
  END IF;
  ALTER TABLE forensic.subscriber_registration DROP CONSTRAINT IF EXISTS subscriber_registration_time_basis_chk;
  ALTER TABLE forensic.subscriber_registration ADD CONSTRAINT subscriber_registration_time_basis_chk
    CHECK (time_basis IN ('declared', 'assumed_by_profile', 'confirmed_by_analyst', 'explicit_offset'));
  ALTER TABLE forensic.subscriber_registration ALTER COLUMN time_basis SET NOT NULL;
END
$$;
COMMIT;
