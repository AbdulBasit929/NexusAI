-- Rollback for 020_contact_links_and_inventory.sql: drops the three views. No table or row was changed by the migration.
-- The Ask engine of an image that knows these views (examples connection-path, connection-path-any and evidence-count, the catalogue snapshot) must be rolled back with them, or its
-- connection questions and its inventory questions are rejected by the validator (the views do not exist); questions of every other class are unaffected.
BEGIN;
DROP VIEW IF EXISTS analyst.v_connection_edges;
DROP VIEW IF EXISTS analyst.v_contact_links;
DROP VIEW IF EXISTS analyst.v_evidence_inventory;
COMMIT;
