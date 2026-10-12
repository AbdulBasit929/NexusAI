-- Destructive rollback for 014_entity_relationship_layer.sql: drops entities, relationships, evidence rows, derivations and
-- IP assignments AND every row in them. These tables are derived data, but analyst review decisions live in relationship:
-- export them first if they matter. Run only from a verified backup, with ON_ERROR_STOP enabled and an explicit:
--   SET app.entity_layer_allow_destructive_rollback = 'true';
BEGIN;

DO $$
DECLARE
  tbl text;
  n bigint;
BEGIN
  IF current_setting('app.entity_layer_allow_destructive_rollback', true) IS DISTINCT FROM 'true' THEN
    RAISE EXCEPTION 'the 014 rollback is destructive; set app.entity_layer_allow_destructive_rollback=true explicitly'
      USING ERRCODE = '55000';
  END IF;

  IF to_regclass('forensic.relationship') IS NOT NULL THEN
    SELECT count(*) INTO n FROM forensic.relationship WHERE review_state <> 'unreviewed';
    IF n > 0 THEN
      RAISE NOTICE '% relationships carry an analyst review decision that this rollback will delete', n;
    END IF;
  END IF;

  FOREACH tbl IN ARRAY ARRAY['relationship_evidence', 'relationship', 'entity_mention', 'entity', 'ip_assignment', 'derivation']
  LOOP
    IF to_regclass('forensic.' || tbl) IS NOT NULL THEN
      EXECUTE format('SELECT count(*) FROM forensic.%I', tbl) INTO n;
      RAISE NOTICE 'dropping forensic.% with % rows', tbl, n;
    END IF;
  END LOOP;
END
$$;

DROP TABLE IF EXISTS forensic.relationship_evidence;
DROP TABLE IF EXISTS forensic.relationship;
DROP TABLE IF EXISTS forensic.entity_mention;
DROP TABLE IF EXISTS forensic.entity;
DROP TABLE IF EXISTS forensic.ip_assignment;
DROP TABLE IF EXISTS forensic.derivation;

COMMIT;
