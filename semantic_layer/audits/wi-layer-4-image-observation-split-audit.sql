-- WI-LAYER-4 read-only oracle for the proposed split of
-- forensics.image-observation/v1. This does not create either entity.

BEGIN READ ONLY;

-- The top-level observation_type is populated on every row and separates the
-- retained corpus into two disjoint record meanings.
SELECT
  metadata ->> 'observation_type' AS observation_type,
  metadata ->> 'source_truth_state' AS source_truth_state,
  count(*) AS row_count,
  count(metadata #>> '{observation,source_metadata}') AS source_metadata_rows,
  count(metadata #>> '{observation,frame_number}') AS frame_number_rows,
  count(metadata #>> '{observation,sampling_interval_seconds}') AS sampling_interval_rows
FROM forensic.derived_artifacts
WHERE collection_id = 'nexusai-multimodal-product-acceptance'
  AND artifact_type = 'forensics.image-observation/v1'
GROUP BY 1, 2
ORDER BY 1, 2;

-- This must remain zero before a discriminator-based split is accepted: a row
-- may not carry both meanings or neither meaning.
WITH rows AS (
  SELECT metadata
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.image-observation/v1'
), classified AS (
  SELECT
    metadata ->> 'observation_type' AS observation_type,
    metadata #>> '{observation,source_metadata}' IS NOT NULL AS has_source_metadata,
    metadata #>> '{observation,frame_number}' IS NOT NULL AS has_frame_number,
    metadata #>> '{observation,sampling_interval_seconds}' IS NOT NULL AS has_sampling_interval,
    metadata #>> '{observation,sampling_strategy}' IS NOT NULL AS has_sampling_strategy,
    metadata #>> '{observation,frame_number_semantics}' IS NOT NULL AS has_frame_semantics
  FROM rows
)
SELECT count(*) AS discriminator_violations
FROM classified
WHERE NOT (
  (
    observation_type = 'image_technical_observation'
    AND has_source_metadata
    AND NOT has_frame_number
    AND NOT has_sampling_interval
    AND NOT has_sampling_strategy
    AND NOT has_frame_semantics
  )
  OR
  (
    observation_type = 'video_sampled_frame_observation'
    AND NOT has_source_metadata
    AND has_frame_number
    AND has_sampling_interval
    AND has_sampling_strategy
    AND has_frame_semantics
  )
);

-- Candidate scalar fields for the still-image technical-metadata entity. Arrays
-- and objects are reported by type but are not proposed as semantic fields.
WITH rows AS (
  SELECT metadata #> '{observation,source_metadata}' AS source_metadata
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.image-observation/v1'
    AND metadata ->> 'observation_type' = 'image_technical_observation'
), field_values AS (
  SELECT key AS field_name, value
  FROM rows
  CROSS JOIN LATERAL jsonb_each(source_metadata)
)
SELECT
  field_name,
  jsonb_typeof(value) AS value_type,
  count(*) AS non_null_count,
  count(DISTINCT value) AS distinct_count,
  min(CASE WHEN jsonb_typeof(value) = 'number' THEN (value #>> '{}')::numeric END) AS numeric_min,
  max(CASE WHEN jsonb_typeof(value) = 'number' THEN (value #>> '{}')::numeric END) AS numeric_max
FROM field_values
GROUP BY field_name, jsonb_typeof(value)
ORDER BY field_name, value_type;

-- Candidate scalar fields for the sampled-video-frame entity. Hashes and parent
-- identifiers are intentionally excluded from the proposed analyst vocabulary.
WITH rows AS (
  SELECT metadata #> '{observation}' AS observation
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.image-observation/v1'
    AND metadata ->> 'observation_type' = 'video_sampled_frame_observation'
), field_values AS (
  SELECT key AS field_name, value
  FROM rows
  CROSS JOIN LATERAL jsonb_each(observation)
  WHERE key NOT IN ('frame_sha256', 'parent_evidence_id', 'parent_version_id')
)
SELECT
  field_name,
  jsonb_typeof(value) AS value_type,
  count(*) AS non_null_count,
  count(DISTINCT value) AS distinct_count,
  min(CASE WHEN jsonb_typeof(value) = 'number' THEN (value #>> '{}')::numeric END) AS numeric_min,
  max(CASE WHEN jsonb_typeof(value) = 'number' THEN (value #>> '{}')::numeric END) AS numeric_max
FROM field_values
GROUP BY field_name, jsonb_typeof(value)
ORDER BY field_name, value_type;

COMMIT;
