-- Read-only verification of optional migration 016 (PostGIS geometry). Run after 016 on an image that ships PostGIS.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  problem text;
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'postgis') THEN
    RAISE EXCEPTION 'the postgis extension is not installed';
  END IF;

  SELECT string_agg(t.name, ', ')
    INTO problem
  FROM (VALUES ('cell_site'), ('camera_site')) AS t(name)
  WHERE NOT EXISTS (
    SELECT 1 FROM pg_attribute a
    WHERE a.attrelid = ('forensic.' || t.name)::regclass AND a.attname = 'geom' AND NOT a.attisdropped AND a.attgenerated = 's');
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'generated geom column missing on: %', problem;
  END IF;

  SELECT string_agg(i.name, ', ')
    INTO problem
  FROM (VALUES ('cell_site_geom_idx'), ('camera_site_geom_idx'), ('plate_sighting_geom_idx')) AS i(name)
  WHERE NOT EXISTS (SELECT 1 FROM pg_indexes x WHERE x.schemaname = 'forensic' AND x.indexname = i.name AND x.indexdef LIKE '%USING gist%');
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'geometry indexes missing: %', problem;
  END IF;

  IF to_regprocedure('forensic.cell_coverage_circle(forensic.cell_site)') IS NULL THEN
    RAISE EXCEPTION 'forensic.cell_coverage_circle is missing';
  END IF;

  -- One degree of longitude on the equator is about 111.3 km on the ellipsoid.
  IF ST_Distance(ST_SetSRID(ST_MakePoint(0, 0), 4326)::geography, ST_SetSRID(ST_MakePoint(1, 0), 4326)::geography)
       NOT BETWEEN 111000 AND 111700 THEN
    RAISE EXCEPTION 'geography distance is not behaving as expected';
  END IF;
END
$$;

SELECT 'migration_016_verified' AS result;

ROLLBACK;
