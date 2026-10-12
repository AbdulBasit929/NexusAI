-- Destructive rollback for 015_vectors_and_earth.sql: drops the embedding table AND every vector in it, and the two earth
-- indexes. Embeddings are derived data and can be recomputed, but that costs model time. Run only from a verified backup,
-- with ON_ERROR_STOP enabled and an explicit:
--   SET app.vectors_allow_destructive_rollback = 'true';
-- The extensions (vector, cube, earthdistance) are NOT dropped: other objects may use them, and an installed extension is
-- harmless. To remove them deliberately, run these by hand after checking that nothing depends on them:
--   DROP EXTENSION earthdistance; DROP EXTENSION cube; DROP EXTENSION vector;
BEGIN;

DO $$
DECLARE
  n bigint;
BEGIN
  IF current_setting('app.vectors_allow_destructive_rollback', true) IS DISTINCT FROM 'true' THEN
    RAISE EXCEPTION 'the 015 rollback is destructive; set app.vectors_allow_destructive_rollback=true explicitly'
      USING ERRCODE = '55000';
  END IF;

  IF to_regclass('forensic.content_embedding') IS NOT NULL THEN
    EXECUTE 'SELECT count(*) FROM forensic.content_embedding' INTO n;
    RAISE NOTICE 'dropping forensic.content_embedding with % rows', n;
  END IF;
END
$$;

DROP INDEX IF EXISTS forensic.camera_site_earth_idx;
DROP INDEX IF EXISTS forensic.cell_site_earth_idx;
DROP TABLE IF EXISTS forensic.content_embedding;

COMMIT;
