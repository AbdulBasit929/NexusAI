-- OPTIONAL migration 016 (DRAFT; syntax-checked only, NEVER run) : PostGIS geometry for cell sites, cameras and plate sightings.
--
-- Do not apply this to the current database. The current image (timescale/timescaledb:latest-pg16, PostgreSQL 16 on Alpine)
-- ships no PostGIS: `SELECT name FROM pg_available_extensions WHERE name = 'postgis'` returns nothing there. Using this file
-- needs a different image, and the move is a dump and restore, not an in-place change, because the Alpine image uses musl and
-- the usual PostGIS images (the postgis/postgis and timescale/timescaledb-ha families; verify the tag before use) use glibc: the
-- two can order text differently, so indexes built on text columns would be wrong if the data directory were reused.
-- Procedure and decision: docs/data/d2-migration-runbook.md, section 9.
--
-- This file lives in optional/ so the Docker init mount (which runs every top-level *.sql in db/forensic_records on a fresh
-- database) never executes it by accident. Migrations 013 and 015 already give radius lookups on cell_site and camera_site
-- through earthdistance; add PostGIS only for true geometry: containment (geofence polygons), line-of-travel, coverage
-- footprints and distance on the ellipsoid.
--
-- Requires 012 to 015. Additive and idempotent. Verify and rollback: 016_postgis_geometry.verify.sql, .rollback.sql.
BEGIN;

CREATE EXTENSION IF NOT EXISTS postgis;

-- Points on the WGS 84 ellipsoid. Generated columns keep geometry in step with lat and lon; the small reference tables can
-- afford the rewrite.
ALTER TABLE forensic.cell_site
  ADD COLUMN IF NOT EXISTS geom geography(Point, 4326)
  GENERATED ALWAYS AS (
    CASE WHEN lat IS NULL OR lon IS NULL THEN NULL
         ELSE ST_SetSRID(ST_MakePoint(lon, lat), 4326)::geography END) STORED;

ALTER TABLE forensic.camera_site
  ADD COLUMN IF NOT EXISTS geom geography(Point, 4326)
  GENERATED ALWAYS AS (
    CASE WHEN lat IS NULL OR lon IS NULL THEN NULL
         ELSE ST_SetSRID(ST_MakePoint(lon, lat), 4326)::geography END) STORED;

CREATE INDEX IF NOT EXISTS cell_site_geom_idx ON forensic.cell_site USING gist (geom);
CREATE INDEX IF NOT EXISTS camera_site_geom_idx ON forensic.camera_site USING gist (geom);

-- plate_sighting is a hypertable that grows: an expression index avoids rewriting it.
CREATE INDEX IF NOT EXISTS plate_sighting_geom_idx
  ON forensic.plate_sighting USING gist ((ST_SetSRID(ST_MakePoint(lon, lat), 4326)::geography))
  WHERE lat IS NOT NULL AND lon IS NOT NULL;

-- Coverage footprint of a site as a circle of the stated radius. A sector wedge from azimuth and beam width is a later
-- refinement; a cell is not a position, and an answer that uses this must show the radius.
CREATE OR REPLACE FUNCTION forensic.cell_coverage_circle(site forensic.cell_site)
RETURNS geography
LANGUAGE sql IMMUTABLE PARALLEL SAFE
AS $$
  SELECT CASE WHEN site.geom IS NULL OR site.radius_m IS NULL THEN NULL
              ELSE ST_Buffer(site.geom, site.radius_m)::geography END
$$;

COMMENT ON COLUMN forensic.cell_site.geom IS 'Point on WGS 84 built from lat and lon (generated).';
COMMENT ON FUNCTION forensic.cell_coverage_circle(forensic.cell_site) IS
  'Circle of radius_m around the site. Not a position of any device; show the radius with any answer that uses it.';

COMMIT;
