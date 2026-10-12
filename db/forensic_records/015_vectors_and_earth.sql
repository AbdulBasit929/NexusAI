-- Migration 015 (DRAFT, not applied): vector search and earth-distance support with the extensions already shipped in the
-- current image (pgvector, cube, earthdistance). No image change and no data movement.
--
-- * content_embedding keeps text and image embeddings next to the evidence they came from. It has no similarity index yet:
--   an approximate index (HNSW) is created per model and dimension once a model is chosen (decision D8); exact search over a
--   case's vectors is fast enough until then. Face embeddings are NOT stored here by design: face comparison stays at
--   policy level L0 (detection only) until the case-authority migration is approved (roadmap R9).
-- * Earth indexes make "cells or cameras within N metres of a point" an index lookup on cell_site and camera_site now. They
--   are spherical approximations; true geometry (PostGIS) is a separate, optional decision (optional/016_postgis_geometry.sql).
--
-- Requires migrations 008 and 013. Additive and idempotent.
BEGIN;
-- Fail fast instead of queueing behind a long transaction: a DDL statement that waits for a lock also blocks every
-- writer queued behind it. If a lock cannot be had within ten seconds the whole migration rolls back; run it again later.
SET LOCAL lock_timeout = '10s';

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS cube;
CREATE EXTENSION IF NOT EXISTS earthdistance;

CREATE TABLE IF NOT EXISTS forensic.content_embedding (
  embedding_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  evidence_id uuid NOT NULL,
  derived_artifact_id uuid,
  modality text NOT NULL,
  chunk_ref text NOT NULL,
  locator jsonb NOT NULL DEFAULT '{}'::jsonb,
  content_sha256 text,
  model_id text NOT NULL,
  model_sha256 text,
  dim integer NOT NULL,
  embedding vector NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT content_embedding_evidence_fk FOREIGN KEY (tenant_id, collection_id, evidence_id)
    REFERENCES forensic.evidence_items (tenant_id, collection_id, evidence_id),
  CONSTRAINT content_embedding_artifact_fk FOREIGN KEY (tenant_id, collection_id, derived_artifact_id)
    REFERENCES forensic.derived_artifacts (tenant_id, collection_id, artifact_id),
  CONSTRAINT content_embedding_modality_chk CHECK (modality IN ('text', 'image')),
  CONSTRAINT content_embedding_dim_chk CHECK (dim > 0 AND dim <= 16000 AND vector_dims(embedding) = dim),
  CONSTRAINT content_embedding_locator_chk CHECK (jsonb_typeof(locator) = 'object'),
  CONSTRAINT content_embedding_hash_chk CHECK (content_sha256 IS NULL OR content_sha256 ~ '^[0-9a-f]{64}$'),
  UNIQUE (tenant_id, collection_id, evidence_id, model_id, modality, chunk_ref)
);

CREATE INDEX IF NOT EXISTS content_embedding_evidence_idx
  ON forensic.content_embedding (tenant_id, collection_id, evidence_id, model_id);

COMMENT ON TABLE forensic.content_embedding IS
  'Text and image embeddings per evidence chunk, tagged with the model that produced them. Face embeddings are excluded by policy.';
COMMENT ON COLUMN forensic.content_embedding.embedding IS
  'Unconstrained vector so several models can coexist; dim records its length. Create one partial HNSW index per model: CREATE INDEX ... USING hnsw ((embedding::vector(N)) vector_cosine_ops) WHERE model_id = ''...'' AND dim = N.';

CREATE INDEX IF NOT EXISTS cell_site_earth_idx
  ON forensic.cell_site USING gist (ll_to_earth(lat, lon)) WHERE lat IS NOT NULL AND lon IS NOT NULL;
CREATE INDEX IF NOT EXISTS camera_site_earth_idx
  ON forensic.camera_site USING gist (ll_to_earth(lat, lon)) WHERE lat IS NOT NULL AND lon IS NOT NULL;

COMMENT ON INDEX forensic.cell_site_earth_idx IS
  'Radius lookup: WHERE earth_box(ll_to_earth(:lat, :lon), :metres) @> ll_to_earth(lat, lon) AND earth_distance(ll_to_earth(:lat, :lon), ll_to_earth(lat, lon)) <= :metres (the box uses the index, the distance removes the corners).';

ALTER TABLE forensic.content_embedding ENABLE ROW LEVEL SECURITY;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = 'content_embedding'
                 AND policyname = 'tenant_isolation_content_embedding') THEN
    CREATE POLICY tenant_isolation_content_embedding ON forensic.content_embedding
      USING (tenant_id = current_setting('app.tenant_id', true))
      WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
  END IF;

  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    GRANT SELECT, INSERT ON forensic.content_embedding TO forensic_runtime;
  END IF;
END
$$;

COMMIT;
