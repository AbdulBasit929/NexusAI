-- Migration 031 (DRAFT, not applied): the mention index of text documents, and three analyst views over it (reference cases programme, stage S3b; design and gates in section 10 of
-- docs/roadmap/reference-cases-programme-20261028.md).
--
-- Why. A seizure memo, a request letter, a witness statement or a chat export names numbers, accounts, plates, handsets, SIMs and places. The document pipeline already keeps the text of every
-- passage (forensic.derived_artifacts); "which documents mention this number", "which documents name both", "do the numbers the documents name appear in the call records" are ordinary questions of
-- an inquiry and had no table to ask. The rule-based extractor (ingestion/forensic_records/mention_extractor.py) reads the passages; this stores what it found.
--
-- What it adds:
--   * forensic.document_mention        one row per mention: which passage of which document, the kind, the normalised key, the text as written, the confidence tier and the rule. Tenant row-level security.
--   * analyst.v_document_mentions      one row per mention, with the document's file name and where in it (paragraph, page or section).
--   * analyst.v_document_values        per document, kind and key: how many times it is named (the thick view: "which documents mention X", "both", "but not").
--   * analyst.v_document_links         per kind and key named in the documents: how many records of another family carry the same key (calls, registrations, wallet, statements, camera
--                                      sightings, handsets), so "do the numbers in the documents appear in the call records" is one read.
--
-- Evidence, not conclusion. A mention says that a document names the key. It does not say the holder of the number took part in anything, that the document is true, or that the person who
-- wrote it knew the holder. A key is compared in its normalised form (a number as 92 and ten digits, an account without spaces and in capitals, a plate as its search key, a place in lower case).
--
-- National ID numbers are COUNTED and WITHHELD. Identity data stays dark until the masking design is approved (R9, as in migration 012): a row of kind national_id exists (the document does
-- contain one) but carries neither the key nor the text; a check constraint keeps it so. The passage text itself is the pipeline's own and unchanged.
--
-- Scope is STRUCTURAL, as in migration 017 (tenant and collection from the transaction settings; unset means no rows). Requires migrations 007, 008, 012, 017, 028 and 030 (the views the
-- link view reads). Idempotent. Apply by hand on a retained database: ops/031_document_mentions/preflight.sql first, verify.sql after; rollback.sql drops the views and the table (only while it is empty).
BEGIN;
SET LOCAL lock_timeout = '10s';

CREATE TABLE IF NOT EXISTS forensic.document_mention (
  mention_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  evidence_id uuid NOT NULL,
  artifact_id uuid NOT NULL,
  kind text NOT NULL,
  value text,
  surface text,
  tier text NOT NULL DEFAULT 'high',
  rule text NOT NULL,
  char_start integer NOT NULL,
  char_end integer NOT NULL,
  locator jsonb NOT NULL DEFAULT '{}'::jsonb,
  extractor text NOT NULL,
  extractor_revision text NOT NULL,
  review_state text NOT NULL DEFAULT 'unreviewed',
  ingested_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT document_mention_evidence_fk FOREIGN KEY (tenant_id, collection_id, evidence_id)
    REFERENCES forensic.evidence_items (tenant_id, collection_id, evidence_id),
  CONSTRAINT document_mention_artifact_fk FOREIGN KEY (artifact_id) REFERENCES forensic.derived_artifacts (artifact_id) ON DELETE CASCADE,
  CONSTRAINT document_mention_kind_chk CHECK (kind IN ('msisdn', 'account', 'plate', 'iccid', 'imei', 'imsi', 'place', 'national_id')),
  CONSTRAINT document_mention_tier_chk CHECK (tier IN ('high', 'low')),
  CONSTRAINT document_mention_review_chk CHECK (review_state IN ('unreviewed', 'confirmed', 'rejected')),
  CONSTRAINT document_mention_span_chk CHECK (char_start >= 0 AND char_end > char_start),
  CONSTRAINT document_mention_locator_chk CHECK (jsonb_typeof(locator) = 'object'),
  CONSTRAINT document_mention_withheld_chk CHECK (
    (kind = 'national_id' AND value IS NULL AND surface IS NULL) OR (kind <> 'national_id' AND value IS NOT NULL AND value <> '' AND surface IS NOT NULL)),
  UNIQUE (tenant_id, collection_id, artifact_id, kind, char_start, char_end, extractor_revision)
);

CREATE INDEX IF NOT EXISTS document_mention_value_idx ON forensic.document_mention (tenant_id, collection_id, kind, value) WHERE value IS NOT NULL;
CREATE INDEX IF NOT EXISTS document_mention_evidence_idx ON forensic.document_mention (tenant_id, collection_id, evidence_id);

COMMENT ON TABLE forensic.document_mention IS
  'What the rule-based extractor found in the text passages of documents: one row per mention (kind, normalised key, the text as written, tier, rule, position). A mention says that a document names the key, nothing more. National ID numbers are counted with no key and no text until the masking design is approved.';

ALTER TABLE forensic.document_mention ENABLE ROW LEVEL SECURITY;
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = 'document_mention' AND policyname = 'tenant_isolation_document_mention') THEN
    CREATE POLICY tenant_isolation_document_mention ON forensic.document_mention USING (tenant_id = current_setting('app.tenant_id', true)) WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    GRANT SELECT, INSERT ON forensic.document_mention TO forensic_runtime;
  END IF;
END
$$;

CREATE OR REPLACE VIEW analyst.v_document_mentions AS
SELECT
  m.mention_id,
  m.evidence_id,
  i.original_filename AS document,
  m.kind,
  m.value,
  m.surface,
  m.tier,
  m.rule,
  COALESCE(m.locator ->> 'paragraph', m.locator ->> 'page', m.locator ->> 'section') AS position_in_document,
  m.char_start,
  m.char_end
FROM forensic.document_mention m
JOIN forensic.evidence_items i
  ON i.tenant_id = m.tenant_id AND i.collection_id = m.collection_id AND i.evidence_id = m.evidence_id
WHERE m.tenant_id = current_setting('app.tenant_id', true)
  AND m.collection_id = current_setting('app.collection_id', true)
  AND m.review_state <> 'rejected'
  AND m.extractor_revision = (SELECT max(x.extractor_revision) FROM forensic.document_mention x
                              WHERE x.tenant_id = m.tenant_id AND x.collection_id = m.collection_id AND x.artifact_id = m.artifact_id);

CREATE OR REPLACE VIEW analyst.v_document_values AS
SELECT
  d.evidence_id,
  d.document,
  d.kind,
  d.value,
  count(*) AS mentions,
  min(d.surface) AS first_surface
FROM analyst.v_document_mentions d
WHERE d.value IS NOT NULL
GROUP BY d.evidence_id, d.document, d.kind, d.value;

CREATE OR REPLACE VIEW analyst.v_document_links AS
WITH named AS (SELECT DISTINCT kind, value FROM analyst.v_document_values)
SELECT x.kind, x.value, x.family, sum(x.records)::bigint AS records
FROM (
  SELECT n.kind, n.value, 'calls'::text AS family, count(*) AS records FROM named n JOIN analyst.v_comm_events e ON e.src = n.value WHERE n.kind = 'msisdn' GROUP BY n.kind, n.value
  UNION ALL
  SELECT n.kind, n.value, 'calls'::text, count(*) FROM named n JOIN analyst.v_comm_events e ON e.dst = n.value WHERE n.kind = 'msisdn' GROUP BY n.kind, n.value
  UNION ALL
  SELECT n.kind, n.value, 'registrations'::text, count(*) FROM named n JOIN analyst.v_subscriber_registrations r ON r.msisdn = n.value WHERE n.kind = 'msisdn' GROUP BY n.kind, n.value
  UNION ALL
  SELECT n.kind, n.value, 'wallet'::text, count(*) FROM named n JOIN analyst.v_bank_transactions t ON t.counterparty_msisdn = n.value WHERE n.kind = 'msisdn' GROUP BY n.kind, n.value
  UNION ALL
  SELECT n.kind, n.value, 'statements'::text, count(*) FROM named n JOIN analyst.v_bank_transactions t ON t.account = n.value WHERE n.kind = 'account' GROUP BY n.kind, n.value
  UNION ALL
  SELECT n.kind, n.value, 'statements'::text, count(*) FROM named n JOIN analyst.v_bank_transactions t ON t.counterparty_account = n.value WHERE n.kind = 'account' GROUP BY n.kind, n.value
  UNION ALL
  SELECT n.kind, n.value, 'sightings'::text, count(*) FROM named n JOIN analyst.v_plate_sightings s ON s.plate_norm = n.value WHERE n.kind = 'plate' GROUP BY n.kind, n.value
  UNION ALL
  SELECT n.kind, n.value, 'registrations'::text, count(*) FROM named n JOIN analyst.v_subscriber_registrations r ON r.iccid = n.value WHERE n.kind = 'iccid' GROUP BY n.kind, n.value
  UNION ALL
  SELECT n.kind, n.value, 'registrations'::text, count(*) FROM named n JOIN analyst.v_subscriber_registrations r ON r.imsi = n.value WHERE n.kind = 'imsi' GROUP BY n.kind, n.value
  UNION ALL
  SELECT n.kind, n.value, 'calls'::text, count(*) FROM named n JOIN analyst.v_comm_events e ON e.imei = n.value WHERE n.kind = 'imei' GROUP BY n.kind, n.value
) x
GROUP BY x.kind, x.value, x.family;

COMMENT ON VIEW analyst.v_document_mentions IS
  'One row per mention found in a text document by the rule-based extractor: the document, the kind (msisdn, account, plate, iccid, imei, imsi, place or national_id), the normalised key, the text as written, the confidence tier and the rule, and where in the document (paragraph, page or section). A mention says that the document names the key. It does not say the holder took part in anything or that the document is true. National ID numbers appear with no key and no text (withheld until the masking design is approved): they can be counted, not looked up.';
COMMENT ON VIEW analyst.v_document_values IS
  'Per document, kind and key: how many times the document names it (the thick view for "which documents mention X", "which mention both X and Y", "how many documents mention X"). Compare value, which is the normalised key. National ID numbers are not here (withheld).';
COMMENT ON VIEW analyst.v_document_links IS
  'Per kind and key named in the documents, how many records of another family carry the same key: calls (a number as caller or called, or a handset), registrations (a number, SIM serial or IMSI), wallet and statements (a counterparty number or an account), sightings (a plate). A key with no row here is named in a document and in no record of the case. The same key in a document and a record is a lead, not a finding.';

DO $$
DECLARE
  d record;
BEGIN
  FOR d IN
    SELECT * FROM (VALUES
      ('v_document_mentions', 'mention_id', 'Identifier of the mention row.'),
      ('v_document_mentions', 'evidence_id', 'The evidence file (the document).'),
      ('v_document_mentions', 'document', 'The document''s file name.'),
      ('v_document_mentions', 'kind', 'What was found: msisdn (a mobile number), account (an IBAN), plate, iccid (a SIM serial), imei, imsi, place (an area of the case''s city) or national_id (counted, withheld).'),
      ('v_document_mentions', 'value', 'The normalised key: a mobile number as 92 and ten digits, an account in capitals without spaces, a plate as its search key, a place in lower case. Empty for national_id. Always compare this column.'),
      ('v_document_mentions', 'surface', 'The text as the document wrote it (the evidence). Empty for national_id. Do not compare it.'),
      ('v_document_mentions', 'tier', 'high (the rule and its cue were met) or low (a weaker reading, for example an IMEI with no cue word).'),
      ('v_document_mentions', 'rule', 'The rule that found it.'),
      ('v_document_mentions', 'position_in_document', 'The paragraph, page or section of the document the mention is in.'),
      ('v_document_mentions', 'char_start', 'Position of the first character in the passage text.'),
      ('v_document_mentions', 'char_end', 'Position after the last character in the passage text.'),
      ('v_document_values', 'evidence_id', 'The evidence file (the document).'),
      ('v_document_values', 'document', 'The document''s file name.'),
      ('v_document_values', 'kind', 'msisdn, account, plate, iccid, imei, imsi or place.'),
      ('v_document_values', 'value', 'The normalised key (see v_document_mentions.value). Always compare this column.'),
      ('v_document_values', 'mentions', 'How many times the document names the key.'),
      ('v_document_values', 'first_surface', 'One of the ways the document wrote it.'),
      ('v_document_links', 'kind', 'msisdn, account, plate, iccid, imei or imsi (a place is not a key of any record).'),
      ('v_document_links', 'value', 'The normalised key named in at least one document.'),
      ('v_document_links', 'family', 'Where the same key was found: calls, registrations, wallet, statements or sightings.'),
      ('v_document_links', 'records', 'How many records of that family carry the key.')
    ) AS t(view_name, column_name, description)
  LOOP
    EXECUTE format('COMMENT ON COLUMN analyst.%I.%I IS %L', d.view_name, d.column_name, d.description);
  END LOOP;
END
$$;

REVOKE ALL ON analyst.v_document_mentions, analyst.v_document_values, analyst.v_document_links FROM PUBLIC;
GRANT SELECT ON analyst.v_document_mentions, analyst.v_document_values, analyst.v_document_links TO forensic_analyst_ro;
COMMIT;
