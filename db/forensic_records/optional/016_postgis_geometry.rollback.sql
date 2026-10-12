-- Rollback for optional migration 016: removes the geometry columns, indexes and the footprint function. The PostGIS extension
-- itself is left installed (other objects may use it). The geometry columns are generated from lat and lon, so nothing is lost.
-- Run with ON_ERROR_STOP enabled and an explicit:
--   SET app.postgis_allow_rollback = 'true';
BEGIN;

DO $$
BEGIN
  IF current_setting('app.postgis_allow_rollback', true) IS DISTINCT FROM 'true' THEN
    RAISE EXCEPTION 'the 016 rollback changes the schema; set app.postgis_allow_rollback=true explicitly' USING ERRCODE = '55000';
  END IF;
END
$$;

DROP FUNCTION IF EXISTS forensic.cell_coverage_circle(forensic.cell_site);
DROP INDEX IF EXISTS forensic.plate_sighting_geom_idx;
DROP INDEX IF EXISTS forensic.camera_site_geom_idx;
DROP INDEX IF EXISTS forensic.cell_site_geom_idx;
ALTER TABLE forensic.camera_site DROP COLUMN IF EXISTS geom;
ALTER TABLE forensic.cell_site DROP COLUMN IF EXISTS geom;

COMMIT;
