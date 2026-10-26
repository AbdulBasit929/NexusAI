-- Migration 027 (DRAFT, not applied): analyst views over the typed plate sightings of camera exports (ANPR), docs/data/anpr-typed-family-20261026.md (P1-B, first family).
--
-- Why. A camera export is a list of plate sightings: "how many times was this plate seen", "where and when", "which cameras", "which plates were seen at both of two cameras" are
-- ordinary questions of a vehicle inquiry. The typed table exists since migration 012 (forensic.plate_sighting, filled by the production typed ingest from the layout anpr-a1 and later layouts);
-- the Ask engine reaches data only through reviewed analyst views (migration 017), so this adds two.
--
-- What it adds (nothing existing is changed; no table is touched):
--   * analyst.v_plate_sightings  one row per sighting of a camera export: the plate as the camera wrote it and its search key, the time, the camera, the lane and the camera's position.
--   * analyst.v_plate_visits     per plate and camera: how many times, first and last, with the camera's name and position (the thick view: "which cameras saw it", "both cameras").
--
-- Plates. plate_norm is the SEARCH KEY (Unicode NFKC, Arabic-Indic digits to ASCII, separators removed, upper case: forensic.normalize_plate). A question's plate is compared in this form
-- (LEA-1234 is 'LEA1234'); plate_raw is what the camera wrote and is the evidence. The key never makes two different plates equal: O is not 0, I is not 1, and a look-alike plate is another plate.
--
-- Camera exports only. source_kind = 'csv': plates read from images or video by a model are another family (ruling C12 of the plan: derived rows are never merged with ingested records) and are not here;
-- rows an analyst rejected (review_state 'rejected') and exact repeats within a file (quality flag duplicate_row, the same rule as every other typed view) are left out.
--
-- A camera sighting says that a camera read this plate at this time. It does not say who drove, and a camera can misread a plate. The position is the camera's, not the vehicle's.
--
-- Scope is STRUCTURAL, as in migration 017: each view carries `tenant_id = current_setting('app.tenant_id', true) AND collection_id = current_setting('app.collection_id', true)`; an unset setting
-- is NULL and the view returns no rows (fail closed). Withheld on purpose: raw payloads, image hashes and boxes, model identities, free text beyond the camera's own name.
--
-- Requires migrations 012 and 017. Idempotent. Apply by hand on a retained database: ops/027_plate_sightings/preflight.sql first, verify.sql after; rollback.sql drops the two views.
BEGIN;
SET LOCAL lock_timeout = '10s';

CREATE OR REPLACE VIEW analyst.v_plate_sightings AS
SELECT
  p.sighting_id,
  p.evidence_id,
  p.source_row,
  p.plate_norm,
  p.plate_raw,
  p.observed_utc,
  p.camera_id,
  p.camera_label,
  p.lane,
  p.lat,
  p.lon,
  p.loc_method,
  p.confidence,
  p.time_basis,
  p.source_tz
FROM forensic.plate_sighting p
WHERE p.tenant_id = current_setting('app.tenant_id', true)
  AND p.collection_id = current_setting('app.collection_id', true)
  AND p.source_kind = 'csv'
  AND p.review_state <> 'rejected'
  AND NOT ('duplicate_row' = ANY (p.quality_flags));

CREATE OR REPLACE VIEW analyst.v_plate_visits AS
SELECT
  s.plate_norm,
  s.camera_id,
  max(s.camera_label) AS camera_label,
  avg(s.lat) AS lat,
  avg(s.lon) AS lon,
  count(*) AS sightings,
  min(s.observed_utc) AS first_utc,
  max(s.observed_utc) AS last_utc
FROM analyst.v_plate_sightings s
GROUP BY s.plate_norm, s.camera_id;

COMMENT ON VIEW analyst.v_plate_sightings IS
  'Plate sightings of camera exports, one row per sighting, repeats within a file left out, times in UTC (shown on the case clock). A sighting says a camera read this plate at this time, not who drove; a camera can misread. Compare plate_norm, never plate_raw. Plates read from images or video are not here.';
COMMENT ON VIEW analyst.v_plate_visits IS
  'Per plate and camera: how many times the plate was seen there, the first and the last time, with the camera''s name and position (the camera''s, not the vehicle''s). For which cameras saw a plate, and which plates were seen at both of two cameras.';

DO $$
DECLARE
  d record;
BEGIN
  FOR d IN
    SELECT * FROM (VALUES
      ('v_plate_sightings', 'sighting_id', 'Identifier of the sighting row.'),
      ('v_plate_sightings', 'evidence_id', 'The evidence file the sighting came from.'),
      ('v_plate_sightings', 'source_row', 'The row of that file.'),
      ('v_plate_sightings', 'plate_norm', 'The search key of the plate: upper case, digits in ASCII, no separators (LEA-1234 is LEA1234). Always compare this column, with the plate written the same way. O is not 0 and I is not 1.'),
      ('v_plate_sightings', 'plate_raw', 'The plate exactly as the camera export wrote it (the evidence). Do not compare it.'),
      ('v_plate_sightings', 'observed_utc', 'Time of the sighting, UTC (shown on the case clock).'),
      ('v_plate_sightings', 'camera_id', 'The camera as the export names it.'),
      ('v_plate_sightings', 'camera_label', 'The camera''s name when the export has one.'),
      ('v_plate_sightings', 'lane', 'The lane when the export has one.'),
      ('v_plate_sightings', 'lat', 'Latitude of the camera when the export has one (the camera, not the vehicle).'),
      ('v_plate_sightings', 'lon', 'Longitude of the camera when the export has one (the camera, not the vehicle).'),
      ('v_plate_sightings', 'loc_method', 'Where the position came from: source_supplied (the export), or none.'),
      ('v_plate_sightings', 'confidence', 'The camera''s own confidence in its reading, from 0 to 1, when the export has one (a percent in the file is converted).'),
      ('v_plate_sightings', 'time_basis', 'How the zone of the time is known: declared, assumed_by_profile, confirmed_by_analyst or explicit_offset.'),
      ('v_plate_sightings', 'source_tz', 'The time zone the export''s times were read in.'),
      ('v_plate_visits', 'plate_norm', 'The search key of the plate (see v_plate_sightings.plate_norm). Always compare this column.'),
      ('v_plate_visits', 'camera_id', 'The camera as the export names it.'),
      ('v_plate_visits', 'camera_label', 'The camera''s name when the export has one.'),
      ('v_plate_visits', 'lat', 'Latitude of the camera (the camera, not the vehicle), empty when unknown.'),
      ('v_plate_visits', 'lon', 'Longitude of the camera (the camera, not the vehicle), empty when unknown.'),
      ('v_plate_visits', 'sightings', 'How many times the plate was seen at the camera.'),
      ('v_plate_visits', 'first_utc', 'Time of the first sighting at the camera, UTC.'),
      ('v_plate_visits', 'last_utc', 'Time of the last sighting at the camera, UTC.')
    ) AS t(view_name, column_name, description)
  LOOP
    EXECUTE format('COMMENT ON COLUMN analyst.%I.%I IS %L', d.view_name, d.column_name, d.description);
  END LOOP;
END
$$;

-- The role reads the new views and nothing else: SELECT is granted view by view; no other privilege on them.
REVOKE ALL ON analyst.v_plate_sightings, analyst.v_plate_visits FROM PUBLIC;
GRANT SELECT ON analyst.v_plate_sightings, analyst.v_plate_visits TO forensic_analyst_ro;
COMMIT;
