-- Migration 019: analyst.v_cells also knows the positions the RECORDS carry, not only the cell reference list (roadmap 7.8.3 row 3, the Places map).
--
-- Why. Migration 017's v_cells reads forensic.cell_site (a reference list of cell sites). A case whose positions are in the files themselves (call-record exports with a latitude and
-- a longitude on every row, tower dumps that list the cell and its position) has no such list, so the map and "which cells did this number use" had cells and no positions: on the
-- real-data collection 1,317 cells are used and 718 of them carry a position in the records, yet v_cell_visits showed none.
--
-- What changes. For a cell that the reference list does not describe, v_cells now returns the position the records give it: the average of the latitudes and longitudes of the case's
-- rows on that cell (repeats left out), and the largest stated uncertainty if the rows state one. A cell the reference list DOES describe is unchanged (the case's own list still
-- wins over the tenant-wide one, and both win over the records). A cell with no position anywhere still has none. A position taken from the records is a coverage area at best,
-- never the position of a person; the view's description says so.
--
-- Same columns and types as before (CREATE OR REPLACE VIEW is refused otherwise), same scope predicate on tenant and collection, same role grants (the grant on the view stays).
-- Additive and idempotent. Apply by hand on a retained database: ops/019_observed_cell_positions/preflight.sql first, verify.sql after; rollback.sql puts migration 017's definition back.
BEGIN;
SET LOCAL lock_timeout = '10s';

CREATE OR REPLACE VIEW analyst.v_cells AS
WITH reference AS (
  SELECT DISTINCT ON (c.cell_key)
    c.cell_key,
    c.site_name,
    c.operator_label AS operator,
    c.lat,
    c.lon,
    c.radius_m
  FROM forensic.cell_site c
  WHERE c.tenant_id = current_setting('app.tenant_id', true)
    AND (c.collection_id = '' OR c.collection_id = current_setting('app.collection_id', true))
    AND c.status = 'active'
  ORDER BY c.cell_key, (c.collection_id = ''), c.source_rank, c.valid_from DESC
), observed AS (
  SELECT
    e.cell_key,
    avg(e.lat)::double precision AS lat,
    avg(e.lon)::double precision AS lon,
    max(e.loc_uncertainty_m)::integer AS radius_m
  FROM forensic.comm_event e
  WHERE e.tenant_id = current_setting('app.tenant_id', true)
    AND e.collection_id = current_setting('app.collection_id', true)
    AND e.cell_key IS NOT NULL AND e.lat IS NOT NULL AND e.lon IS NOT NULL
    AND e.lat BETWEEN -90 AND 90 AND e.lon BETWEEN -180 AND 180
    AND NOT ('duplicate_row' = ANY (e.quality_flags))
  GROUP BY e.cell_key
)
SELECT r.cell_key, r.site_name, r.operator, r.lat, r.lon, r.radius_m FROM reference r
UNION ALL
SELECT o.cell_key, NULL::text, NULL::text, o.lat, o.lon, o.radius_m
FROM observed o
WHERE NOT EXISTS (SELECT 1 FROM reference r WHERE r.cell_key = o.cell_key);

COMMENT ON VIEW analyst.v_cells IS 'Cell sites known to the case: name, position and the stated coverage radius. The case''s own list wins, then the tenant-wide one; for a cell neither describes, the position is the average of the positions the records themselves give it (no name, and a radius only if the records state one). A cell is not a position of a person; show the radius with any location claim.';

COMMIT;
