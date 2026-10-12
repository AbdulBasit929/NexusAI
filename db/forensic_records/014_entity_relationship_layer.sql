-- Migration 014 (DRAFT, not applied): entities, relationships and the evidence behind them.
--
-- The link-analysis foundation (roadmap section 7). Entities are things with strong keys (phone, IMSI, IMEI, address, plate,
-- account, cell, application, domain, evidence item); a person exists only when an analyst creates one, and a name is never a
-- key. Relationships connect entities and always carry a kind:
--   observed  - a record says it;
--   inferred  - a documented rule derived it from observed facts (must point at a derivation row);
--   candidate - a model or heuristic suggested it and no person has confirmed it (must point at a derivation row).
-- Every relationship is explainable to rows through relationship_evidence, and every inferred or candidate one records how
-- it was produced in derivation (procedure, parameters, code version, seed).
--
-- Face comparison is deliberately absent: no face_candidate relationship type exists until the case-authority migration
-- (roadmap R9, policy level L1 and above) is approved.
--
-- Additive and idempotent. Requires migrations 007 and 008 (evidence registry). Independent of 012 and 013.
BEGIN;
-- Fail fast instead of queueing behind a long transaction: a DDL statement that waits for a lock also blocks every
-- writer queued behind it. If a lock cannot be had within ten seconds the whole migration rolls back; run it again later.
SET LOCAL lock_timeout = '10s';

-- ---------------------------------------------------------------------------------------------------------------------
-- derivation: how an inferred edge, a score or a derived table row was produced.
-- ---------------------------------------------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS forensic.derivation (
  derivation_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  procedure text NOT NULL,
  parameters jsonb NOT NULL DEFAULT '{}'::jsonb,
  code_version text NOT NULL,
  input_fingerprint text,
  seed bigint,
  run_by text NOT NULL DEFAULT '',
  run_at timestamptz NOT NULL DEFAULT now(),
  notes text,
  CONSTRAINT derivation_procedure_chk CHECK (procedure <> '' AND code_version <> ''),
  CONSTRAINT derivation_parameters_chk CHECK (jsonb_typeof(parameters) = 'object'),
  UNIQUE (tenant_id, collection_id, derivation_id)
);

CREATE INDEX IF NOT EXISTS derivation_procedure_idx
  ON forensic.derivation (tenant_id, collection_id, procedure, run_at DESC);

COMMENT ON TABLE forensic.derivation IS
  'One row per run of an analysis procedure: what ran, with which parameters, code version and seed, over which inputs.';

-- ---------------------------------------------------------------------------------------------------------------------
-- entity
-- ---------------------------------------------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS forensic.entity (
  entity_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  entity_type text NOT NULL,
  canonical text NOT NULL,
  display_label text,
  origin text NOT NULL DEFAULT 'system',
  first_seen timestamptz,
  last_seen timestamptz,
  attributes jsonb NOT NULL DEFAULT '{}'::jsonb,
  merged_into uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT entity_type_chk CHECK (entity_type IN
    ('phone', 'imsi', 'imei', 'ip', 'plate', 'account', 'cell', 'device', 'person', 'application', 'domain', 'evidence')),
  CONSTRAINT entity_origin_chk CHECK (origin IN ('system', 'analyst')),
  CONSTRAINT entity_canonical_chk CHECK (canonical <> ''),
  CONSTRAINT entity_person_chk CHECK (entity_type <> 'person' OR (origin = 'analyst' AND canonical LIKE 'person:%')),
  CONSTRAINT entity_seen_chk CHECK (first_seen IS NULL OR last_seen IS NULL OR last_seen >= first_seen),
  CONSTRAINT entity_attributes_chk CHECK (jsonb_typeof(attributes) = 'object'),
  CONSTRAINT entity_not_self_merged_chk CHECK (merged_into IS NULL OR merged_into <> entity_id),
  UNIQUE (tenant_id, collection_id, entity_type, canonical),
  UNIQUE (tenant_id, collection_id, entity_id)
);

ALTER TABLE forensic.entity DROP CONSTRAINT IF EXISTS entity_merged_fk;
ALTER TABLE forensic.entity
  ADD CONSTRAINT entity_merged_fk FOREIGN KEY (tenant_id, collection_id, merged_into)
  REFERENCES forensic.entity (tenant_id, collection_id, entity_id)
  DEFERRABLE INITIALLY DEFERRED;

CREATE INDEX IF NOT EXISTS entity_type_idx
  ON forensic.entity (tenant_id, collection_id, entity_type);
CREATE INDEX IF NOT EXISTS entity_canonical_trgm_idx
  ON forensic.entity USING gin (canonical gin_trgm_ops);

DROP TRIGGER IF EXISTS entity_touch_updated_at ON forensic.entity;
CREATE TRIGGER entity_touch_updated_at
  BEFORE UPDATE ON forensic.entity
  FOR EACH ROW EXECUTE FUNCTION forensic.touch_updated_at();

COMMENT ON TABLE forensic.entity IS
  'Things with strong keys. canonical is the normalised key (digits for a phone, the plate search key, the evidence id for an evidence item). A person is created only by an analyst, with a generated person:<id> key; a name is never a key.';
COMMENT ON COLUMN forensic.entity.merged_into IS
  'Set when an analyst decides two entities are the same; the merged entity is kept, never deleted, so earlier answers stay explainable.';

-- ---------------------------------------------------------------------------------------------------------------------
-- entity_mention: where an entity was found in documents, speech, images and other non-tabular evidence.
-- Tabular rows are not mirrored here: their typed columns are the mentions, and relationship rows summarise them.
-- ---------------------------------------------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS forensic.entity_mention (
  mention_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  entity_id uuid NOT NULL,
  evidence_id uuid NOT NULL,
  derived_artifact_id uuid,
  source_table text NOT NULL,
  source_key text NOT NULL,
  role text NOT NULL DEFAULT 'mentioned',
  observed_at timestamptz,
  confidence real,
  locator jsonb NOT NULL DEFAULT '{}'::jsonb,
  snippet text,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT entity_mention_entity_fk FOREIGN KEY (tenant_id, collection_id, entity_id)
    REFERENCES forensic.entity (tenant_id, collection_id, entity_id),
  CONSTRAINT entity_mention_evidence_fk FOREIGN KEY (tenant_id, collection_id, evidence_id)
    REFERENCES forensic.evidence_items (tenant_id, collection_id, evidence_id),
  CONSTRAINT entity_mention_artifact_fk FOREIGN KEY (tenant_id, collection_id, derived_artifact_id)
    REFERENCES forensic.derived_artifacts (tenant_id, collection_id, artifact_id),
  CONSTRAINT entity_mention_role_chk CHECK (role IN ('mentioned', 'speaker', 'subject', 'sender', 'recipient', 'owner', 'other')),
  CONSTRAINT entity_mention_confidence_chk CHECK (confidence IS NULL OR confidence BETWEEN 0 AND 1),
  CONSTRAINT entity_mention_locator_chk CHECK (jsonb_typeof(locator) = 'object'),
  UNIQUE (tenant_id, collection_id, entity_id, source_table, source_key, role)
);

CREATE INDEX IF NOT EXISTS entity_mention_entity_idx
  ON forensic.entity_mention (tenant_id, collection_id, entity_id, observed_at);
CREATE INDEX IF NOT EXISTS entity_mention_evidence_idx
  ON forensic.entity_mention (tenant_id, collection_id, evidence_id);

COMMENT ON TABLE forensic.entity_mention IS
  'Entity found in a document, transcript, image text or similar. Carries the OCR or speech confidence and a locator (page, box, time range).';
COMMENT ON COLUMN forensic.entity_mention.snippet IS
  'Short context for display. May contain personal data; treat it with the evidence it came from.';

-- ---------------------------------------------------------------------------------------------------------------------
-- relationship
-- ---------------------------------------------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS forensic.relationship (
  rel_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  src_entity uuid NOT NULL,
  dst_entity uuid NOT NULL,
  rel_type text NOT NULL,
  kind text NOT NULL,
  directed boolean NOT NULL DEFAULT true,
  first_seen timestamptz,
  last_seen timestamptz,
  evidence_count bigint NOT NULL DEFAULT 0,
  features jsonb NOT NULL DEFAULT '{}'::jsonb,
  score real,
  derivation_id uuid,
  review_state text NOT NULL DEFAULT 'unreviewed',
  reviewed_by text,
  reviewed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT relationship_src_fk FOREIGN KEY (tenant_id, collection_id, src_entity)
    REFERENCES forensic.entity (tenant_id, collection_id, entity_id),
  CONSTRAINT relationship_dst_fk FOREIGN KEY (tenant_id, collection_id, dst_entity)
    REFERENCES forensic.entity (tenant_id, collection_id, entity_id),
  CONSTRAINT relationship_derivation_fk FOREIGN KEY (tenant_id, collection_id, derivation_id)
    REFERENCES forensic.derivation (tenant_id, collection_id, derivation_id),
  CONSTRAINT relationship_distinct_ends_chk CHECK (src_entity <> dst_entity),
  CONSTRAINT relationship_undirected_order_chk CHECK (directed OR src_entity < dst_entity),
  CONSTRAINT relationship_kind_chk CHECK (kind IN ('observed', 'inferred', 'candidate')),
  CONSTRAINT relationship_type_kind_chk CHECK (
    (kind = 'observed' AND rel_type IN ('called', 'texted', 'contacted', 'used_device', 'used_sim', 'at_cell', 'used_app',
                                        'visited_domain', 'mentioned_in', 'paid'))
    OR (kind = 'inferred' AND rel_type IN ('co_located', 'co_traveled', 'shared_ip'))
    OR (kind = 'candidate' AND rel_type IN ('successor_candidate'))),
  CONSTRAINT relationship_derivation_required_chk CHECK (kind = 'observed' OR derivation_id IS NOT NULL),
  CONSTRAINT relationship_seen_chk CHECK (first_seen IS NULL OR last_seen IS NULL OR last_seen >= first_seen),
  CONSTRAINT relationship_evidence_count_chk CHECK (evidence_count >= 0),
  CONSTRAINT relationship_features_chk CHECK (jsonb_typeof(features) = 'object'),
  CONSTRAINT relationship_review_chk CHECK (review_state IN ('unreviewed', 'confirmed', 'rejected')),
  UNIQUE (tenant_id, collection_id, src_entity, dst_entity, rel_type, kind),
  UNIQUE (tenant_id, collection_id, rel_id)
);

CREATE INDEX IF NOT EXISTS relationship_src_idx
  ON forensic.relationship (tenant_id, collection_id, src_entity, rel_type);
CREATE INDEX IF NOT EXISTS relationship_dst_idx
  ON forensic.relationship (tenant_id, collection_id, dst_entity, rel_type);
CREATE INDEX IF NOT EXISTS relationship_derivation_idx
  ON forensic.relationship (tenant_id, collection_id, derivation_id) WHERE derivation_id IS NOT NULL;

DROP TRIGGER IF EXISTS relationship_touch_updated_at ON forensic.relationship;
CREATE TRIGGER relationship_touch_updated_at
  BEFORE UPDATE ON forensic.relationship
  FOR EACH ROW EXECUTE FUNCTION forensic.touch_updated_at();

COMMENT ON TABLE forensic.relationship IS
  'Typed edge between two entities. observed = a record says it; inferred = a documented rule derived it; candidate = suggested, unconfirmed. Inferred and candidate edges must name their derivation.';
COMMENT ON COLUMN forensic.relationship.directed IS
  'False for symmetric relationships (co_located, shared_ip, co_traveled): stored once with src_entity < dst_entity.';
COMMENT ON COLUMN forensic.relationship.score IS
  'Documented link-strength formula output, always shown with its features; never a replacement for the evidence list.';

-- ---------------------------------------------------------------------------------------------------------------------
-- relationship_evidence: the rows that justify an edge.
-- ---------------------------------------------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS forensic.relationship_evidence (
  rel_id uuid NOT NULL,
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  source_table text NOT NULL,
  source_key text NOT NULL,
  observed_at timestamptz,
  weight real NOT NULL DEFAULT 1,
  PRIMARY KEY (rel_id, source_table, source_key),
  CONSTRAINT relationship_evidence_rel_fk FOREIGN KEY (tenant_id, collection_id, rel_id)
    REFERENCES forensic.relationship (tenant_id, collection_id, rel_id) ON DELETE CASCADE,
  CONSTRAINT relationship_evidence_source_chk CHECK (source_table IN
    ('comm_event', 'data_session', 'app_usage', 'plate_sighting', 'subscriber_registration', 'entity_mention', 'ip_assignment')),
  CONSTRAINT relationship_evidence_weight_chk CHECK (weight >= 0)
);

CREATE INDEX IF NOT EXISTS relationship_evidence_row_idx
  ON forensic.relationship_evidence (tenant_id, collection_id, source_table, source_key);

COMMENT ON TABLE forensic.relationship_evidence IS
  'One row per source row behind an edge. source_key is the primary key of the source row as text (event_id, session_id, ...).';

-- ---------------------------------------------------------------------------------------------------------------------
-- ip_assignment: who held a public address and port range during an interval (derived from data_session).
-- Rebuildable: every row names the derivation that produced it.
-- ---------------------------------------------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS forensic.ip_assignment (
  assignment_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  public_ip inet NOT NULL,
  port_from integer,
  port_to integer,
  valid_from timestamptz NOT NULL,
  valid_to timestamptz NOT NULL,
  msisdn text,
  imsi text,
  imei text,
  source_key text NOT NULL,
  derivation_id uuid NOT NULL,
  CONSTRAINT ip_assignment_derivation_fk FOREIGN KEY (tenant_id, collection_id, derivation_id)
    REFERENCES forensic.derivation (tenant_id, collection_id, derivation_id),
  CONSTRAINT ip_assignment_ports_chk CHECK (
    (port_from IS NULL AND port_to IS NULL)
    OR (port_from IS NOT NULL AND port_to IS NOT NULL AND port_from BETWEEN 0 AND 65535 AND port_to BETWEEN port_from AND 65535)),
  CONSTRAINT ip_assignment_interval_chk CHECK (valid_to >= valid_from),
  CONSTRAINT ip_assignment_holder_chk CHECK (msisdn IS NOT NULL OR imsi IS NOT NULL OR imei IS NOT NULL),
  UNIQUE (tenant_id, collection_id, derivation_id, source_key)
);

CREATE INDEX IF NOT EXISTS ip_assignment_lookup_idx
  ON forensic.ip_assignment (tenant_id, collection_id, public_ip, valid_from, valid_to);

COMMENT ON TABLE forensic.ip_assignment IS
  'Interval lookup for IP attribution: WHERE public_ip = x AND port in range AND time within the interval. More than one holder means the answer is ambiguous and is reported as such.';

-- ---------------------------------------------------------------------------------------------------------------------
-- Row-level security and privileges. Column-level UPDATE grants keep identity and key columns immutable for the runtime role.
-- ---------------------------------------------------------------------------------------------------------------------
DO $$
DECLARE
  t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['derivation', 'entity', 'entity_mention', 'relationship', 'relationship_evidence', 'ip_assignment']
  LOOP
    EXECUTE format('ALTER TABLE forensic.%I ENABLE ROW LEVEL SECURITY', t);
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = t AND policyname = 'tenant_isolation_' || t) THEN
      EXECUTE format(
        'CREATE POLICY %I ON forensic.%I USING (tenant_id = current_setting(''app.tenant_id'', true)) WITH CHECK (tenant_id = current_setting(''app.tenant_id'', true))',
        'tenant_isolation_' || t, t);
    END IF;
  END LOOP;

  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    GRANT SELECT, INSERT ON forensic.derivation, forensic.entity, forensic.entity_mention, forensic.relationship,
      forensic.relationship_evidence, forensic.ip_assignment TO forensic_runtime;
    GRANT UPDATE (display_label, first_seen, last_seen, attributes, merged_into, updated_at) ON forensic.entity TO forensic_runtime;
    GRANT UPDATE (first_seen, last_seen, evidence_count, features, score, review_state, reviewed_by, reviewed_at, updated_at)
      ON forensic.relationship TO forensic_runtime;
  END IF;
END
$$;

COMMIT;
