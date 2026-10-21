-- Migration 021: the case record (decision D30; backend requests 18 and 23 of docs/work/BACKEND_REQUESTS.md, minimal form).
--
-- A case is still a collection_id on the evidence rows; this table is the ONE place that says what the analysts call it and whether it is open.
-- It stores a display name, a lifecycle status, who created the record and who last changed it. It does NOT hold members, an owner, a
-- classification or a priority: those wait for per-user identity (master plan P5) and must not be invented before there is a source for them.
-- A case with no row is reported as open with no display name; nothing is backfilled and nothing is guessed (created_by stays NULL = "not recorded").
--
-- Additive and idempotent. Nothing references the table and it references nothing.
BEGIN;
SET LOCAL lock_timeout = '10s';

CREATE TABLE IF NOT EXISTS forensic.case_record (
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  display_name text,
  status text NOT NULL DEFAULT 'open',
  created_by text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_by text,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, collection_id),
  CONSTRAINT case_record_status_chk CHECK (status IN ('open', 'on_hold', 'closed')),
  CONSTRAINT case_record_collection_chk CHECK (collection_id <> '' AND length(collection_id) <= 128),
  CONSTRAINT case_record_name_chk CHECK (display_name IS NULL OR (length(btrim(display_name)) BETWEEN 1 AND 120 AND display_name !~ '[[:cntrl:]]'))
);

COMMENT ON TABLE forensic.case_record IS
  'One row per case that has been named or had its status changed: display name, open/on_hold/closed, who created the record and who last changed it. No membership, owner or classification (master plan D30).';
COMMENT ON COLUMN forensic.case_record.display_name IS
  'Free text chosen by an analyst, 1 to 120 characters, no control characters. Always shown escaped, with the case identifier kept visible beside it.';
COMMENT ON COLUMN forensic.case_record.created_by IS
  'The subject that first recorded the case. NULL means not recorded: it is never inferred from evidence rows.';

ALTER TABLE forensic.case_record ENABLE ROW LEVEL SECURITY;
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = 'case_record' AND policyname = 'tenant_isolation_case_record') THEN
    CREATE POLICY tenant_isolation_case_record ON forensic.case_record
      USING (tenant_id = current_setting('app.tenant_id', true))
      WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    GRANT SELECT, INSERT ON forensic.case_record TO forensic_runtime;
    GRANT UPDATE (display_name, status, updated_by, updated_at) ON forensic.case_record TO forensic_runtime;
  END IF;
END
$$;

COMMIT;
