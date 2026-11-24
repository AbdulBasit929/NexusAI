-- Migration 038 (DRAFT, not applied): image search over a whole case (module M1 of docs/roadmap/media-modules-reconciled-plan-20261120.md; gates in docs/data/image-search-m1-preregistration-20261120.md).
--
-- Why. Today an analyst can compare two NAMED images (image_compare.go, a 64-bit hash) or rank at most 200 NAMED images by SigLIP similarity (image_similarity.go); nothing searches a whole case, and the vectors sit
-- inside observation payloads. This migration gives the search a place to read from and a place to leave its trace:
--   1. forensic.image_hash        - the 256-bit PDQ-style perceptual hash of every image (and later video frame) of the current version of an evidence item. One row per evidence, version and chunk.
--   2. forensic.image_search_log  - one row per search: who, when, what kind, the probe (hash of the uploaded image, or the query text), the case, the parameters, how many results, how much of the case was indexed.
--   3. Four read functions (SECURITY INVOKER, so row-level security applies, STABLE, exact scan inside ONE collection, current evidence versions only):
--        forensic.image_index_state(collection, case)                       - images in the case, indexed, failed, identical re-uploads (the "coverage line" of every answer)
--        forensic.image_search_hash(collection, probe bit(256), max_distance, ...)       - "the same image, re-saved": Hamming distance on the PDQ-style hash
--        forensic.image_search_vector(collection, model_id, probe vector, max_distance, ...) - cosine distance over forensic.content_embedding of one model (DINOv2 "same photograph, edited";
--                                                                                  SigLIP 2 "photos by words" / "more like this")
--        forensic.image_exact_matches(collection, sha256, ...)           - the identical file (same SHA-256) under another evidence id
--   The vectors themselves stay in forensic.content_embedding (migration 015, modality 'image'; one row per model and chunk, chunk_ref 'v:<version>:full'). No similarity index is created: exact search inside a case
--   is the design (22 ms for 1,000 rows, 169 ms for 10,000 measured 2026-11-19), and exactness is what lets an answer say it is complete.
--
-- What it does not do: it creates no face table (module M2), no cross-case search, and no index. Additive and idempotent; nothing existing changes except one grant list (runtime_role.grants.sql).
-- Rollback: ops/038_image_search/rollback.sql (gated; drops the two tables and the four functions, never content_embedding). Apply by hand on a retained database: ops/038_image_search/preflight.sql first,
-- verify.sql after, and only after a backup and a rehearsal on a restored copy (ops/Test-MigrationsThrowaway.ps1 and smoke.sql for disposable databases).
-- Requires migrations 008 and 015.
BEGIN;
SET LOCAL lock_timeout = '10s';

CREATE TABLE IF NOT EXISTS forensic.image_hash (
  hash_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  evidence_id uuid NOT NULL,
  version_id uuid NOT NULL,
  derived_artifact_id uuid NOT NULL,
  chunk_ref text NOT NULL DEFAULT 'full',
  algorithm text NOT NULL,
  pdq256 bit(256) NOT NULL,
  image_width integer,
  image_height integer,
  content_sha256 text,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT image_hash_evidence_fk FOREIGN KEY (tenant_id, collection_id, evidence_id)
    REFERENCES forensic.evidence_items (tenant_id, collection_id, evidence_id),
  CONSTRAINT image_hash_artifact_fk FOREIGN KEY (tenant_id, collection_id, derived_artifact_id)
    REFERENCES forensic.derived_artifacts (tenant_id, collection_id, artifact_id),
  CONSTRAINT image_hash_size_chk CHECK ((image_width IS NULL OR image_width > 0) AND (image_height IS NULL OR image_height > 0)),
  CONSTRAINT image_hash_sha_chk CHECK (content_sha256 IS NULL OR content_sha256 ~ '^[0-9a-f]{64}$'),
  UNIQUE (tenant_id, collection_id, evidence_id, version_id, algorithm, chunk_ref)
);

CREATE INDEX IF NOT EXISTS image_hash_collection_idx ON forensic.image_hash (tenant_id, collection_id, evidence_id);

COMMENT ON TABLE forensic.image_hash IS
  'PDQ-style 256-bit perceptual hash per image or video frame of one version of an evidence item. A match within the calibrated Hamming distance means "the same image, re-saved, resized or recompressed"; it is a candidate for review, never proof of identity.';

CREATE TABLE IF NOT EXISTS forensic.image_search_log (
  search_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  case_id text,
  searched_by text NOT NULL,
  kind text NOT NULL,
  probe_evidence_id uuid,
  probe_sha256 text,
  query_text text,
  query_language text,
  parameters jsonb NOT NULL DEFAULT '{}'::jsonb,
  result_count integer NOT NULL DEFAULT 0,
  images_in_case bigint,
  images_indexed bigint,
  elapsed_ms integer,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT image_search_log_kind_chk CHECK (kind IN ('copies', 'copies_of_upload', 'by_words', 'more_like_this')),
  CONSTRAINT image_search_log_params_chk CHECK (jsonb_typeof(parameters) = 'object'),
  CONSTRAINT image_search_log_sha_chk CHECK (probe_sha256 IS NULL OR probe_sha256 ~ '^[0-9a-f]{64}$'),
  CONSTRAINT image_search_log_query_chk CHECK (query_text IS NULL OR char_length(query_text) <= 512)
);

CREATE INDEX IF NOT EXISTS image_search_log_case_idx ON forensic.image_search_log (tenant_id, collection_id, created_at DESC);

COMMENT ON TABLE forensic.image_search_log IS
  'One row per image search: who searched, when, how, with which probe or phrase, over which case, and how much of the case was indexed at that moment. Append-only for the application role.';

ALTER TABLE forensic.image_hash ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.image_search_log ENABLE ROW LEVEL SECURITY;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = 'image_hash' AND policyname = 'tenant_isolation_image_hash') THEN
    CREATE POLICY tenant_isolation_image_hash ON forensic.image_hash
      USING (tenant_id = current_setting('app.tenant_id', true))
      WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = 'image_search_log' AND policyname = 'tenant_isolation_image_search_log') THEN
    CREATE POLICY tenant_isolation_image_search_log ON forensic.image_search_log
      USING (tenant_id = current_setting('app.tenant_id', true))
      WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
  END IF;
END
$$;

-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------- the coverage line

CREATE OR REPLACE FUNCTION forensic.image_index_state(p_collection text, p_case text DEFAULT NULL)
RETURNS TABLE (images_in_case bigint, images_indexed bigint, images_failed bigint, images_identical_reupload bigint)
LANGUAGE sql STABLE AS $$
  WITH imgs AS (
    SELECT e.evidence_id, e.current_version_id, e.processing_status
    FROM forensic.evidence_items e
    WHERE e.tenant_id = current_setting('app.tenant_id', true)
      AND e.collection_id = p_collection
      AND e.modality = 'image'
      AND e.current_version_id IS NOT NULL
      AND (p_case IS NULL OR e.case_id = p_case)
  )
  SELECT
    count(*) FILTER (WHERE i.processing_status <> 'duplicate')::bigint,
    count(*) FILTER (WHERE i.processing_status <> 'duplicate' AND EXISTS (
      SELECT 1 FROM forensic.image_hash h
      WHERE h.tenant_id = current_setting('app.tenant_id', true) AND h.collection_id = p_collection
        AND h.evidence_id = i.evidence_id AND h.version_id = i.current_version_id))::bigint,
    count(*) FILTER (WHERE i.processing_status = 'failed')::bigint,
    count(*) FILTER (WHERE i.processing_status = 'duplicate')::bigint
  FROM imgs i
$$;

COMMENT ON FUNCTION forensic.image_index_state(text, text) IS
  'The coverage line of every image search: images in the case (re-uploads of an identical file are counted apart), how many have a hash for their current version, how many failed processing.';

-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------- hash tier

CREATE OR REPLACE FUNCTION forensic.image_search_hash(
  p_collection text, p_probe bit(256), p_max_distance integer,
  p_case text DEFAULT NULL, p_exclude_evidence uuid DEFAULT NULL, p_limit integer DEFAULT 1000)
RETURNS TABLE (evidence_id uuid, version_id uuid, derived_artifact_id uuid, chunk_ref text, original_filename text, content_type text,
               sha256 text, case_id text, distance integer, citation_ref text)
LANGUAGE sql STABLE AS $$
  SELECT h.evidence_id, h.version_id, h.derived_artifact_id, h.chunk_ref, e.original_filename, e.content_type, e.sha256, e.case_id,
         bit_count(h.pdq256 # p_probe)::integer AS distance,
         'nexusai://evidence/' || h.evidence_id::text || '/artifacts/' || h.derived_artifact_id::text AS citation_ref
  FROM forensic.image_hash h
  JOIN forensic.evidence_items e
    ON e.tenant_id = h.tenant_id AND e.collection_id = h.collection_id AND e.evidence_id = h.evidence_id AND e.current_version_id = h.version_id
  WHERE h.tenant_id = current_setting('app.tenant_id', true)
    AND h.collection_id = p_collection
    AND (p_case IS NULL OR e.case_id = p_case)
    AND (p_exclude_evidence IS NULL OR h.evidence_id <> p_exclude_evidence)
    AND bit_count(h.pdq256 # p_probe) <= p_max_distance
  ORDER BY 9, h.evidence_id, h.chunk_ref
  LIMIT least(greatest(p_limit, 1), 5000)
$$;

COMMENT ON FUNCTION forensic.image_search_hash(text, bit, integer, text, uuid, integer) IS
  'Every image of the collection whose PDQ-style hash is within p_max_distance bits of the probe (current versions only), nearest first. Exact scan; the caller states the distance it calibrated.';

-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------- vector tier

CREATE OR REPLACE FUNCTION forensic.image_search_vector(
  p_collection text, p_model_id text, p_probe vector, p_max_distance double precision,
  p_case text DEFAULT NULL, p_exclude_evidence uuid DEFAULT NULL, p_limit integer DEFAULT 200)
RETURNS TABLE (evidence_id uuid, version_id uuid, derived_artifact_id uuid, chunk_ref text, original_filename text, content_type text,
               sha256 text, case_id text, distance double precision, citation_ref text)
LANGUAGE sql STABLE AS $$
  WITH scoped AS MATERIALIZED (
    SELECT ce.evidence_id, a.version_id, ce.derived_artifact_id, ce.chunk_ref, e.original_filename, e.content_type, e.sha256, e.case_id, ce.embedding
    FROM forensic.content_embedding ce
    JOIN forensic.derived_artifacts a
      ON a.tenant_id = ce.tenant_id AND a.collection_id = ce.collection_id AND a.artifact_id = ce.derived_artifact_id
    JOIN forensic.evidence_items e
      ON e.tenant_id = ce.tenant_id AND e.collection_id = ce.collection_id AND e.evidence_id = ce.evidence_id AND e.current_version_id = a.version_id
    WHERE ce.tenant_id = current_setting('app.tenant_id', true)
      AND ce.collection_id = p_collection
      AND ce.modality = 'image'
      AND ce.model_id = p_model_id
      AND ce.dim = vector_dims(p_probe)
      AND (p_case IS NULL OR e.case_id = p_case)
      AND (p_exclude_evidence IS NULL OR ce.evidence_id <> p_exclude_evidence)
  )
  SELECT s.evidence_id, s.version_id, s.derived_artifact_id, s.chunk_ref, s.original_filename, s.content_type, s.sha256, s.case_id,
         (s.embedding <=> p_probe)::double precision AS distance,
         'nexusai://evidence/' || s.evidence_id::text || '/artifacts/' || s.derived_artifact_id::text AS citation_ref
  FROM scoped s
  WHERE (s.embedding <=> p_probe) <= p_max_distance
  ORDER BY 9, s.evidence_id, s.chunk_ref
  LIMIT least(greatest(p_limit, 1), 5000)
$$;

COMMENT ON FUNCTION forensic.image_search_vector(text, text, vector, double precision, text, uuid, integer) IS
  'Images of the collection whose vector of one model is within p_max_distance cosine distance of the probe (current versions only), nearest first. Exact scan. The caller passes the model id and its own calibrated distance.';

-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------- identical file

CREATE OR REPLACE FUNCTION forensic.image_exact_matches(
  p_collection text, p_sha256 text, p_case text DEFAULT NULL, p_exclude_evidence uuid DEFAULT NULL, p_limit integer DEFAULT 1000)
RETURNS TABLE (evidence_id uuid, original_filename text, content_type text, sha256 text, case_id text, processing_status text)
LANGUAGE sql STABLE AS $$
  SELECT e.evidence_id, e.original_filename, e.content_type, e.sha256, e.case_id, e.processing_status
  FROM forensic.evidence_items e
  WHERE e.tenant_id = current_setting('app.tenant_id', true)
    AND e.collection_id = p_collection
    AND e.sha256 = lower(p_sha256)
    AND e.modality = 'image'
    AND (p_case IS NULL OR e.case_id = p_case)
    AND (p_exclude_evidence IS NULL OR e.evidence_id <> p_exclude_evidence)
  ORDER BY e.created_at, e.evidence_id
  LIMIT least(greatest(p_limit, 1), 5000)
$$;

COMMENT ON FUNCTION forensic.image_exact_matches(text, text, text, uuid, integer) IS
  'Image evidence of the collection with exactly this file hash: the same file stored under more than one evidence id.';

-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------- privileges

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    GRANT SELECT, INSERT ON forensic.image_hash TO forensic_runtime;
    GRANT SELECT, INSERT ON forensic.image_search_log TO forensic_runtime;
    GRANT USAGE, SELECT ON SEQUENCE forensic.image_hash_hash_id_seq TO forensic_runtime;
    GRANT EXECUTE ON FUNCTION forensic.image_index_state(text, text) TO forensic_runtime;
    GRANT EXECUTE ON FUNCTION forensic.image_search_hash(text, bit, integer, text, uuid, integer) TO forensic_runtime;
    GRANT EXECUTE ON FUNCTION forensic.image_search_vector(text, text, vector, double precision, text, uuid, integer) TO forensic_runtime;
    GRANT EXECUTE ON FUNCTION forensic.image_exact_matches(text, text, text, uuid, integer) TO forensic_runtime;
  END IF;
END
$$;

COMMIT;
