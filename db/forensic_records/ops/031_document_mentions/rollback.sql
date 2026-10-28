-- Rollback for 031_document_mentions.sql: drops the three views and forensic.document_mention (the table only while it is empty).
-- A table that holds rows is NEVER dropped: the whole script is one transaction that stops with a message and changes nothing, and the rows are exported and reviewed first.
-- An Ask engine whose catalogue lists these views (and the document examples) must be rolled back with them, or questions that name them are rejected by the validator.
BEGIN;
DO $$
DECLARE
  has_rows boolean := false;
BEGIN
  IF to_regclass('forensic.document_mention') IS NOT NULL THEN
    EXECUTE 'SELECT EXISTS (SELECT 1 FROM forensic.document_mention)' INTO has_rows;
    IF has_rows THEN
      RAISE EXCEPTION 'forensic.document_mention holds rows: nothing was changed; export and review them before a rollback';
    END IF;
  END IF;
END
$$;
DROP VIEW IF EXISTS analyst.v_document_links;
DROP VIEW IF EXISTS analyst.v_document_values;
DROP VIEW IF EXISTS analyst.v_document_mentions;
DROP TABLE IF EXISTS forensic.document_mention;
COMMIT;
