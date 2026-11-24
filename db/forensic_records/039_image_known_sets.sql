-- Migration 039 (DRAFT, not applied): known sets for images, the watch lists (design and gates: docs/data/image-known-sets-preregistration-20261124.md).
--
-- What. A known set is a named list of pictures known in advance, kept as HASHES ONLY (the SHA-256 of the file and the 256-bit PDQ-style hash of migration 038), of one of two kinds:
-- "of_interest" (a watch list: flag these wherever they turn up) and "known_harmless" (noise to push out of the way: stock photos, logos, app screenshots). Every picture of a case is checked
-- against the active sets from the hashes already stored for it; a match is a FLAG, never a deletion, and hiding flagged pictures is a view choice made in the interface.
--   1. forensic.image_known_set   - the sets (per tenant, not per case, so a watch list works across cases). A set is retired, never deleted.
--   2. forensic.image_known_hash  - the hashes of a set: sha256 and/or pdq256, a short label, where it came from (csv, evidence, upload, api; a file name or an evidence id, never content), who added it.
--                                   No column can hold picture bytes; label and source_ref are limited to 512 characters.
--   3. forensic.image_known_event - append-only trail: set created, hashes added, set retired, and every check of a case against the sets (who, which case, how many hashes, how many flags).
--   4. forensic.image_known_matches(collection, max_distance, case, set_ids) - SECURITY INVOKER, STABLE, exact scan inside ONE collection, current evidence versions only: the flags, strongest tier
--      per picture and set ("identical" = same SHA-256; "same_image" = hash within the distance, the calibrated 56 bits of the copy search).
--      forensic.image_known_state(collection, case) - the coverage line: active sets, hashes in them, images in the case, images that have a hash.
--
-- What it does not do: no pictures are stored, no vectors (a DINOv2 tier would need them and would end "hash-only"), no cross-tenant sharing, no change to evidence, no index (exact scan, like 038).
-- Additive and idempotent. Rollback: ops/039_image_known_sets/rollback.sql (gated; drops the three tables and the two functions; the events are an audit trail, so export them first).
-- Requires migration 038 (forensic.image_hash) and 008 (forensic.evidence_items).
BEGIN;
SET LOCAL lock_timeout = '10s';

CREATE TABLE IF NOT EXISTS forensic.image_known_set (
  set_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  name text NOT NULL,
  kind text NOT NULL,
  description text NOT NULL DEFAULT '',
  created_by text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  retired_at timestamptz,
  retired_by text,
  CONSTRAINT image_known_set_kind_chk CHECK (kind IN ('of_interest', 'known_harmless')),
  CONSTRAINT image_known_set_name_chk CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
  CONSTRAINT image_known_set_description_chk CHECK (char_length(description) <= 1000),
  CONSTRAINT image_known_set_actor_chk CHECK (btrim(created_by) <> ''),
  CONSTRAINT image_known_set_retired_chk CHECK ((retired_at IS NULL) = (retired_by IS NULL)),
  UNIQUE (tenant_id, set_id)
);

-- one ACTIVE set per name within a tenant (a retired set frees its name)
CREATE UNIQUE INDEX IF NOT EXISTS image_known_set_active_name_idx ON forensic.image_known_set (tenant_id, lower(btrim(name))) WHERE retired_at IS NULL;

COMMENT ON TABLE forensic.image_known_set IS
  'A named list of known pictures (hashes only) of one kind: of_interest (watch list) or known_harmless. Per tenant; retired, never deleted; changes are recorded in forensic.image_known_event.';

CREATE TABLE IF NOT EXISTS forensic.image_known_hash (
  known_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id text NOT NULL DEFAULT 'default',
  set_id uuid NOT NULL,
  sha256 text,
  pdq256 bit(256),
  label text NOT NULL DEFAULT '',
  source text NOT NULL,
  source_ref text,
  added_by text NOT NULL,
  added_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT image_known_hash_set_fk FOREIGN KEY (tenant_id, set_id) REFERENCES forensic.image_known_set (tenant_id, set_id),
  CONSTRAINT image_known_hash_any_chk CHECK (sha256 IS NOT NULL OR pdq256 IS NOT NULL),
  CONSTRAINT image_known_hash_sha_chk CHECK (sha256 IS NULL OR sha256 ~ '^[0-9a-f]{64}$'),
  CONSTRAINT image_known_hash_source_chk CHECK (source IN ('csv', 'evidence', 'upload', 'api')),
  CONSTRAINT image_known_hash_label_chk CHECK (char_length(label) <= 512),
  CONSTRAINT image_known_hash_ref_chk CHECK (source_ref IS NULL OR char_length(source_ref) <= 512),
  CONSTRAINT image_known_hash_actor_chk CHECK (btrim(added_by) <> '')
);

-- the same pair of hashes is stored once per set
CREATE UNIQUE INDEX IF NOT EXISTS image_known_hash_unique_idx ON forensic.image_known_hash (set_id, coalesce(sha256, ''), coalesce(pdq256::text, ''));
CREATE INDEX IF NOT EXISTS image_known_hash_sha_idx ON forensic.image_known_hash (tenant_id, sha256) WHERE sha256 IS NOT NULL;
CREATE INDEX IF NOT EXISTS image_known_hash_set_idx ON forensic.image_known_hash (tenant_id, set_id);

COMMENT ON TABLE forensic.image_known_hash IS
  'The hashes of a known set: SHA-256 of the file and/or the 256-bit PDQ-style hash. Hashes and short labels only; the picture itself is never stored. Insert-only for the application role.';

CREATE TABLE IF NOT EXISTS forensic.image_known_event (
  event_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id text NOT NULL DEFAULT 'default',
  set_id uuid,
  kind text NOT NULL,
  actor text NOT NULL,
  collection_id text,
  case_id text,
  detail jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT image_known_event_kind_chk CHECK (kind IN ('set_created', 'hashes_added', 'set_retired', 'check')),
  CONSTRAINT image_known_event_actor_chk CHECK (btrim(actor) <> ''),
  CONSTRAINT image_known_event_detail_chk CHECK (jsonb_typeof(detail) = 'object'),
  CONSTRAINT image_known_event_set_fk FOREIGN KEY (tenant_id, set_id) REFERENCES forensic.image_known_set (tenant_id, set_id)
);

CREATE INDEX IF NOT EXISTS image_known_event_idx ON forensic.image_known_event (tenant_id, created_at DESC);

COMMENT ON TABLE forensic.image_known_event IS
  'Append-only trail of known sets: a set created, hashes added, a set retired, and each check of a case against the sets, with the actor and the counts.';

ALTER TABLE forensic.image_known_set ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.image_known_hash ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.image_known_event ENABLE ROW LEVEL SECURITY;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = 'image_known_set' AND policyname = 'tenant_isolation_image_known_set') THEN
    CREATE POLICY tenant_isolation_image_known_set ON forensic.image_known_set
      USING (tenant_id = current_setting('app.tenant_id', true))
      WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = 'image_known_hash' AND policyname = 'tenant_isolation_image_known_hash') THEN
    CREATE POLICY tenant_isolation_image_known_hash ON forensic.image_known_hash
      USING (tenant_id = current_setting('app.tenant_id', true))
      WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = 'image_known_event' AND policyname = 'tenant_isolation_image_known_event') THEN
    CREATE POLICY tenant_isolation_image_known_event ON forensic.image_known_event
      USING (tenant_id = current_setting('app.tenant_id', true))
      WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
  END IF;
END
$$;

-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------- the flags

CREATE OR REPLACE FUNCTION forensic.image_known_matches(
  p_collection text, p_max_distance integer DEFAULT 56, p_case text DEFAULT NULL, p_set_ids uuid[] DEFAULT NULL)
RETURNS TABLE (evidence_id uuid, set_id uuid, set_name text, set_kind text, tier text, distance integer, label text)
LANGUAGE sql STABLE AS $$
  WITH sets AS (
    SELECT s.set_id, s.name, s.kind
    FROM forensic.image_known_set s
    WHERE s.tenant_id = current_setting('app.tenant_id', true)
      AND s.retired_at IS NULL
      AND (p_set_ids IS NULL OR s.set_id = ANY (p_set_ids))
  ), imgs AS (
    SELECT e.evidence_id, e.current_version_id, e.sha256
    FROM forensic.evidence_items e
    WHERE e.tenant_id = current_setting('app.tenant_id', true)
      AND e.collection_id = p_collection
      AND e.modality = 'image'
      AND e.current_version_id IS NOT NULL
      AND (p_case IS NULL OR e.case_id = p_case)
  ), exact AS (
    SELECT i.evidence_id, k.set_id, 'identical'::text AS tier, 0 AS distance, k.label
    FROM imgs i
    JOIN forensic.image_known_hash k ON k.tenant_id = current_setting('app.tenant_id', true) AND k.sha256 = i.sha256
    JOIN sets s ON s.set_id = k.set_id
  ), perceptual AS (
    SELECT i.evidence_id, k.set_id, 'same_image'::text AS tier, bit_count(h.pdq256 # k.pdq256)::integer AS distance, k.label
    FROM imgs i
    JOIN forensic.image_hash h
      ON h.tenant_id = current_setting('app.tenant_id', true) AND h.collection_id = p_collection AND h.evidence_id = i.evidence_id AND h.version_id = i.current_version_id
    JOIN forensic.image_known_hash k ON k.tenant_id = current_setting('app.tenant_id', true) AND k.pdq256 IS NOT NULL
    JOIN sets s ON s.set_id = k.set_id
    WHERE bit_count(h.pdq256 # k.pdq256) <= p_max_distance
  ), best AS (
    SELECT DISTINCT ON (m.evidence_id, m.set_id) m.evidence_id, m.set_id, m.tier, m.distance, m.label
    FROM (SELECT * FROM exact UNION ALL SELECT * FROM perceptual) m
    ORDER BY m.evidence_id, m.set_id, m.distance, m.tier
  )
  SELECT b.evidence_id, b.set_id, s.name, s.kind, b.tier, b.distance, b.label
  FROM best b
  JOIN sets s ON s.set_id = b.set_id
  ORDER BY b.evidence_id, s.kind, s.name
$$;

COMMENT ON FUNCTION forensic.image_known_matches(text, integer, text, uuid[]) IS
  'The flags of a case: every image of the collection (current versions) that is the identical file of a known hash (tier identical) or whose PDQ-style hash is within p_max_distance bits of a known hash (tier same_image), per active set, the strongest tier per picture and set. Exact scan; the caller states the distance.';

CREATE OR REPLACE FUNCTION forensic.image_known_state(p_collection text, p_case text DEFAULT NULL)
RETURNS TABLE (active_sets bigint, known_hashes bigint, images_in_case bigint, images_with_hash bigint)
LANGUAGE sql STABLE AS $$
  SELECT
    (SELECT count(*) FROM forensic.image_known_set s WHERE s.tenant_id = current_setting('app.tenant_id', true) AND s.retired_at IS NULL)::bigint,
    (SELECT count(*) FROM forensic.image_known_hash k JOIN forensic.image_known_set s ON s.tenant_id = k.tenant_id AND s.set_id = k.set_id
      WHERE k.tenant_id = current_setting('app.tenant_id', true) AND s.retired_at IS NULL)::bigint,
    (SELECT i.images_in_case FROM forensic.image_index_state(p_collection, p_case) i),
    (SELECT i.images_indexed FROM forensic.image_index_state(p_collection, p_case) i)
$$;

COMMENT ON FUNCTION forensic.image_known_state(text, text) IS
  'The coverage line of a known-set check: active sets, the hashes in them, the images of the case, and how many of those have a hash for their current version (the others are checked by SHA-256 only).';

-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------- privileges

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    GRANT SELECT, INSERT ON forensic.image_known_set TO forensic_runtime;
    GRANT UPDATE (retired_at, retired_by) ON forensic.image_known_set TO forensic_runtime;
    GRANT SELECT, INSERT ON forensic.image_known_hash TO forensic_runtime;
    GRANT SELECT, INSERT ON forensic.image_known_event TO forensic_runtime;
    GRANT USAGE, SELECT ON SEQUENCE forensic.image_known_hash_known_id_seq TO forensic_runtime;
    GRANT USAGE, SELECT ON SEQUENCE forensic.image_known_event_event_id_seq TO forensic_runtime;
    GRANT EXECUTE ON FUNCTION forensic.image_known_matches(text, integer, text, uuid[]) TO forensic_runtime;
    GRANT EXECUTE ON FUNCTION forensic.image_known_state(text, text) TO forensic_runtime;
  END IF;
END
$$;

COMMIT;
