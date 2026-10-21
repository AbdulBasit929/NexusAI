-- Read-only compatibility gate for migration 020 (the thick views v_contact_links and v_connection_edges, and v_evidence_inventory). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  missing text;
BEGIN
  IF to_regclass('analyst.v_contact_pairs') IS NULL OR to_regclass('analyst.v_comm_events') IS NULL THEN
    RAISE EXCEPTION 'migration 017 must be applied before migration 020 (analyst.v_contact_pairs or analyst.v_comm_events is missing)';
  END IF;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_contact_pairs'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     NOT LIKE 'src,dst,is_service,is_hub,events,%' THEN
    RAISE EXCEPTION 'analyst.v_contact_pairs does not have the columns migration 017 gave it (src, dst, is_service, is_hub, events ...)';
  END IF;
  SELECT string_agg(c.name, ', ') INTO missing
  FROM (VALUES ('evidence_id'), ('tenant_id'), ('collection_id'), ('original_filename'), ('modality'), ('detected_type'), ('extension'), ('size_bytes'), ('processing_status'), ('created_at')) AS c(name)
  WHERE NOT EXISTS (SELECT 1 FROM information_schema.columns k WHERE k.table_schema = 'forensic' AND k.table_name = 'evidence_items' AND k.column_name = c.name);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION 'forensic.evidence_items lacks the columns the inventory view reads: %', missing;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_analyst_ro') THEN
    RAISE EXCEPTION 'the role forensic_analyst_ro is missing (migration 017)';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = 'analyst' AND c.relname IN ('v_contact_links', 'v_connection_edges', 'v_evidence_inventory') AND c.relkind <> 'v') THEN
    RAISE EXCEPTION 'a relation named like a new view exists in schema analyst and is not a view';
  END IF;
  RAISE NOTICE 'migration 020 preflight passed';
END
$$;

ROLLBACK;
