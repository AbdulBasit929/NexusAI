-- Destructive rollback for 038_image_search.sql: drops the four search functions, forensic.image_hash AND every hash in it, and forensic.image_search_log AND every search recorded in it.
-- Hashes are derived data and can be recomputed (80 ms per image); the search log is an audit trail and cannot, so export it first if it matters. forensic.content_embedding (migration 015) is NOT touched:
-- the vectors the worker wrote there stay, and the search functions are the only thing that read them. Run only from a verified backup, with ON_ERROR_STOP enabled and an explicit:
--   SET app.image_search_allow_destructive_rollback = 'true';
BEGIN;

DO $$
DECLARE
  hashes bigint := 0;
  searches bigint := 0;
BEGIN
  IF current_setting('app.image_search_allow_destructive_rollback', true) IS DISTINCT FROM 'true' THEN
    RAISE EXCEPTION 'the 038 rollback is destructive; set app.image_search_allow_destructive_rollback=true explicitly'
      USING ERRCODE = '55000';
  END IF;
  IF to_regclass('forensic.image_hash') IS NOT NULL THEN
    EXECUTE 'SELECT count(*) FROM forensic.image_hash' INTO hashes;
  END IF;
  IF to_regclass('forensic.image_search_log') IS NOT NULL THEN
    EXECUTE 'SELECT count(*) FROM forensic.image_search_log' INTO searches;
  END IF;
  RAISE NOTICE 'dropping forensic.image_hash with % rows and forensic.image_search_log with % rows', hashes, searches;
END
$$;

DROP FUNCTION IF EXISTS forensic.image_exact_matches(text, text, text, uuid, integer);
DROP FUNCTION IF EXISTS forensic.image_search_vector(text, text, vector, double precision, text, uuid, integer);
DROP FUNCTION IF EXISTS forensic.image_search_hash(text, bit, integer, text, uuid, integer);
DROP FUNCTION IF EXISTS forensic.image_index_state(text, text);
DROP TABLE IF EXISTS forensic.image_search_log;
DROP TABLE IF EXISTS forensic.image_hash;

COMMIT;
