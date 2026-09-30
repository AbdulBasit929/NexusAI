-- WI-LAYER-8 read-only oracle for proposed distinct-capable media fields.
-- The selected values are non-PII technical/model-observation vocabularies.

BEGIN READ ONLY;

WITH field_values AS (
  SELECT
    'image_ocr_observation.script_family' AS field_id,
    metadata #>> '{observation,script_family}' AS value
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.image-ocr-observation/v1'
  UNION ALL
  SELECT
    'face_model_observation.embedding_model',
    metadata #>> '{observation,embedding_model}'
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.face-observation/v1'
  UNION ALL
  SELECT
    'image_fingerprint_observation.perceptual_hash_algorithm',
    metadata #>> '{observation,perceptual_hash_algorithm}'
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.image-fingerprint/v1'
  UNION ALL
  SELECT
    'audio_timestamp_segment.detected_language',
    metadata #>> '{observation,detected_language}'
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.audio-timestamp-segment/v1'
)
SELECT
  field_id,
  count(*) AS retained_rows,
  count(value) AS populated_rows,
  count(DISTINCT value) AS distinct_non_null_values,
  count(*) FILTER (WHERE value IS NULL) AS unavailable_rows
FROM field_values
GROUP BY field_id
ORDER BY field_id;

WITH field_values AS (
  SELECT
    'image_ocr_observation.script_family' AS field_id,
    metadata #>> '{observation,script_family}' AS value
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.image-ocr-observation/v1'
  UNION ALL
  SELECT
    'face_model_observation.embedding_model',
    metadata #>> '{observation,embedding_model}'
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.face-observation/v1'
  UNION ALL
  SELECT
    'image_fingerprint_observation.perceptual_hash_algorithm',
    metadata #>> '{observation,perceptual_hash_algorithm}'
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.image-fingerprint/v1'
  UNION ALL
  SELECT
    'audio_timestamp_segment.detected_language',
    metadata #>> '{observation,detected_language}'
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.audio-timestamp-segment/v1'
)
SELECT field_id, value, count(*) AS rows
FROM field_values
WHERE value IS NOT NULL
GROUP BY field_id, value
ORDER BY field_id, value;

COMMIT;
