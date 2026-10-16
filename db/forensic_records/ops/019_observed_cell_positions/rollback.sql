-- Rollback for 019_observed_cell_positions.sql: puts analyst.v_cells back to migration 017's definition (the reference list only). No table or row was changed by the migration.
-- Whatever drew positions from the records (the Places map, a question about the cells a number used) shows no position for a cell that the reference list does not describe.
BEGIN;
CREATE OR REPLACE VIEW analyst.v_cells AS
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
ORDER BY c.cell_key, (c.collection_id = ''), c.source_rank, c.valid_from DESC;
COMMENT ON VIEW analyst.v_cells IS 'Cell sites known to the case: name, position and the stated coverage radius. A cell is not a position of a person; show the radius with any location claim.';
COMMIT;
