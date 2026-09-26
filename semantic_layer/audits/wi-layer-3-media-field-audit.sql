-- WI-LAYER-3 read-only field oracle.
-- Run against collection nexusai-multimodal-product-acceptance. Each transaction
-- binds one semantic entity to one exact, versioned artifact type and reports only
-- counts/ranges; PII values are never selected.

BEGIN READ ONLY;
WITH rows AS (
  SELECT artifact_id, metadata
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.anpr-observation/v1'
), field_values AS (
  SELECT r.artifact_id, v.field_name, v.value, v.is_number
  FROM rows r
  CROSS JOIN LATERAL (VALUES
    ('plate_text', r.metadata #>> '{observation,normalized_plate_text}', false),
    ('ocr_confidence', r.metadata #>> '{observation,ocr_confidence}', true),
    ('detection_confidence', r.metadata #>> '{observation,detection_confidence}', true),
    ('crop_size_score', r.metadata #>> '{observation,crop_quality}', true),
    ('manual_review_required', r.metadata #>> '{observation,manual_review_required}', false),
    ('interpolated', r.metadata #>> '{observation,interpolated}', false),
    ('source_truth_state', r.metadata ->> 'source_truth_state', false)
  ) AS v(field_name, value, is_number)
)
SELECT
  'anpr_model_observation' AS entity,
  (SELECT count(*) FROM rows) AS matched_rows,
  field_name,
  count(value) AS non_null_count,
  round(100.0 * count(value) / NULLIF(count(*), 0), 1) AS presence_percent,
  count(DISTINCT value) AS distinct_count,
  min(CASE WHEN is_number AND value IS NOT NULL THEN value::numeric END) AS numeric_min,
  max(CASE WHEN is_number AND value IS NOT NULL THEN value::numeric END) AS numeric_max
FROM field_values
GROUP BY field_name
ORDER BY field_name;
COMMIT;

BEGIN READ ONLY;
WITH rows AS (
  SELECT artifact_id, metadata
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.video-anpr-plate-group/v1'
), field_values AS (
  SELECT r.artifact_id, v.field_name, v.value, v.is_number
  FROM rows r
  CROSS JOIN LATERAL (VALUES
    ('plate_text', r.metadata #>> '{observation,normalized_plate_text}', false),
    ('first_seen_seconds', r.metadata #>> '{observation,first_seen_seconds}', true),
    ('last_seen_seconds', r.metadata #>> '{observation,last_seen_seconds}', true),
    ('sightings_count', r.metadata #>> '{observation,sightings_count}', true),
    ('manual_review_required', r.metadata #>> '{observation,manual_review_required}', false),
    ('persistent_tracking', r.metadata #>> '{observation,persistent_tracking}', false),
    ('interpolated_observations', r.metadata #>> '{observation,interpolated_observations}', false),
    ('source_truth_state', r.metadata ->> 'source_truth_state', false)
  ) AS v(field_name, value, is_number)
)
SELECT
  'video_anpr_plate_group' AS entity,
  (SELECT count(*) FROM rows) AS matched_rows,
  field_name,
  count(value) AS non_null_count,
  round(100.0 * count(value) / NULLIF(count(*), 0), 1) AS presence_percent,
  count(DISTINCT value) AS distinct_count,
  min(CASE WHEN is_number AND value IS NOT NULL THEN value::numeric END) AS numeric_min,
  max(CASE WHEN is_number AND value IS NOT NULL THEN value::numeric END) AS numeric_max
FROM field_values
GROUP BY field_name
ORDER BY field_name;
COMMIT;

BEGIN READ ONLY;
WITH rows AS (
  SELECT artifact_id, metadata
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.image-ocr-observation/v1'
), field_values AS (
  SELECT r.artifact_id, v.field_name, v.value, v.is_number
  FROM rows r
  CROSS JOIN LATERAL (VALUES
    ('raw_text', r.metadata #>> '{observation,raw_text}', false),
    ('normalized_text', r.metadata #>> '{observation,normalized_text}', false),
    ('general_ocr_confidence', r.metadata #>> '{observation,confidence}', true),
    ('recognition_confidence', r.metadata #>> '{observation,recognition_confidence}', true),
    ('detection_confidence', r.metadata #>> '{observation,detection_confidence}', true),
    ('script_family', r.metadata #>> '{observation,script_family}', false),
    ('language_claim', r.metadata #>> '{observation,language_claim}', false),
    ('frame_timestamp_seconds', r.metadata #>> '{observation,frame_timestamp_seconds}', true),
    ('review_state', r.metadata #>> '{observation,review_state}', false),
    ('manual_review_required', r.metadata #>> '{observation,manual_review_required}', false),
    ('source_truth_state', r.metadata ->> 'source_truth_state', false)
  ) AS v(field_name, value, is_number)
)
SELECT
  'image_ocr_observation' AS entity,
  (SELECT count(*) FROM rows) AS matched_rows,
  field_name,
  count(value) AS non_null_count,
  round(100.0 * count(value) / NULLIF(count(*), 0), 1) AS presence_percent,
  count(DISTINCT value) AS distinct_count,
  min(CASE WHEN is_number AND value IS NOT NULL THEN value::numeric END) AS numeric_min,
  max(CASE WHEN is_number AND value IS NOT NULL THEN value::numeric END) AS numeric_max
FROM field_values
GROUP BY field_name
ORDER BY field_name;
COMMIT;

BEGIN READ ONLY;
WITH rows AS (
  SELECT artifact_id, metadata
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.audio-timestamp-segment/v1'
), field_values AS (
  SELECT r.artifact_id, v.field_name, v.value, v.is_number
  FROM rows r
  CROSS JOIN LATERAL (VALUES
    ('text', r.metadata #>> '{observation,text}', false),
    ('start_seconds', r.metadata #>> '{observation,start_seconds}', true),
    ('end_seconds', r.metadata #>> '{observation,end_seconds}', true),
    ('detected_language', r.metadata #>> '{observation,detected_language}', false),
    ('timing_status', r.metadata #>> '{observation,timing_status}', false),
    ('manual_review_required', r.metadata #>> '{observation,manual_review_required}', false),
    ('source_truth_state', r.metadata ->> 'source_truth_state', false)
  ) AS v(field_name, value, is_number)
)
SELECT
  'audio_timestamp_segment' AS entity,
  (SELECT count(*) FROM rows) AS matched_rows,
  field_name,
  count(value) AS non_null_count,
  round(100.0 * count(value) / NULLIF(count(*), 0), 1) AS presence_percent,
  count(DISTINCT value) AS distinct_count,
  min(CASE WHEN is_number AND value IS NOT NULL THEN value::numeric END) AS numeric_min,
  max(CASE WHEN is_number AND value IS NOT NULL THEN value::numeric END) AS numeric_max
FROM field_values
GROUP BY field_name
ORDER BY field_name;
COMMIT;

BEGIN READ ONLY;
WITH rows AS (
  SELECT artifact_id, metadata
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.audio-roman-urdu-segment/v1'
), field_values AS (
  SELECT r.artifact_id, v.field_name, v.value, v.is_number
  FROM rows r
  CROSS JOIN LATERAL (VALUES
    ('raw_urdu_text', r.metadata #>> '{observation,raw_urdu_text}', false),
    ('roman_urdu_text', r.metadata #>> '{observation,roman_urdu_text}', false),
    ('start_seconds', r.metadata #>> '{observation,start_seconds}', true),
    ('end_seconds', r.metadata #>> '{observation,end_seconds}', true),
    ('source_language', r.metadata #>> '{observation,source_language}', false),
    ('representation_language', r.metadata #>> '{observation,representation_language}', false),
    ('authority_state', r.metadata #>> '{observation,authority_state}', false),
    ('identifier_preservation_pass', r.metadata #>> '{observation,identifier_preservation_pass}', false),
    ('manual_review_required', r.metadata #>> '{observation,manual_review_required}', false),
    ('source_truth_state', r.metadata ->> 'source_truth_state', false)
  ) AS v(field_name, value, is_number)
)
SELECT
  'audio_roman_urdu_segment' AS entity,
  (SELECT count(*) FROM rows) AS matched_rows,
  field_name,
  count(value) AS non_null_count,
  round(100.0 * count(value) / NULLIF(count(*), 0), 1) AS presence_percent,
  count(DISTINCT value) AS distinct_count,
  min(CASE WHEN is_number AND value IS NOT NULL THEN value::numeric END) AS numeric_min,
  max(CASE WHEN is_number AND value IS NOT NULL THEN value::numeric END) AS numeric_max
FROM field_values
GROUP BY field_name
ORDER BY field_name;
COMMIT;

BEGIN READ ONLY;
WITH rows AS (
  SELECT artifact_id, metadata
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.face-observation/v1'
), field_values AS (
  SELECT r.artifact_id, v.field_name, v.value, v.is_number
  FROM rows r
  CROSS JOIN LATERAL (VALUES
    ('detection_confidence', r.metadata #>> '{observation,detection_confidence}', true),
    ('crop_width_pixels', r.metadata #>> '{observation,quality,crop_width_pixels}', true),
    ('crop_height_pixels', r.metadata #>> '{observation,quality,crop_height_pixels}', true),
    ('minimum_crop_dimension_pixels', r.metadata #>> '{observation,quality,minimum_dimension_pixels}', true),
    ('embedding_dimension', r.metadata #>> '{observation,embedding_dimension}', true),
    ('embedding_model', r.metadata #>> '{observation,embedding_model}', false),
    ('embedding_status', r.metadata #>> '{observation,embedding_status}', false),
    ('review_state', r.metadata #>> '{observation,review_state}', false),
    ('source_truth_state', r.metadata ->> 'source_truth_state', false)
  ) AS v(field_name, value, is_number)
)
SELECT
  'face_model_observation' AS entity,
  (SELECT count(*) FROM rows) AS matched_rows,
  field_name,
  count(value) AS non_null_count,
  round(100.0 * count(value) / NULLIF(count(*), 0), 1) AS presence_percent,
  count(DISTINCT value) AS distinct_count,
  min(CASE WHEN is_number AND value IS NOT NULL THEN value::numeric END) AS numeric_min,
  max(CASE WHEN is_number AND value IS NOT NULL THEN value::numeric END) AS numeric_max
FROM field_values
GROUP BY field_name
ORDER BY field_name;
COMMIT;

BEGIN READ ONLY;
WITH rows AS (
  SELECT artifact_id, metadata
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.image-fingerprint/v1'
), field_values AS (
  SELECT r.artifact_id, v.field_name, v.value, v.is_number
  FROM rows r
  CROSS JOIN LATERAL (VALUES
    ('perceptual_hash_algorithm', r.metadata #>> '{observation,perceptual_hash_algorithm}', false),
    ('perceptual_hash_bits', r.metadata #>> '{observation,perceptual_hash_bits}', true),
    ('near_duplicate_semantics', r.metadata #>> '{observation,near_duplicate_semantics}', false),
    ('exact_identity_algorithm', r.metadata #>> '{observation,exact_identity_algorithm}', false),
    ('source_truth_state', r.metadata ->> 'source_truth_state', false)
  ) AS v(field_name, value, is_number)
)
SELECT
  'image_fingerprint_observation' AS entity,
  (SELECT count(*) FROM rows) AS matched_rows,
  field_name,
  count(value) AS non_null_count,
  round(100.0 * count(value) / NULLIF(count(*), 0), 1) AS presence_percent,
  count(DISTINCT value) AS distinct_count,
  min(CASE WHEN is_number AND value IS NOT NULL THEN value::numeric END) AS numeric_min,
  max(CASE WHEN is_number AND value IS NOT NULL THEN value::numeric END) AS numeric_max
FROM field_values
GROUP BY field_name
ORDER BY field_name;
COMMIT;

BEGIN READ ONLY;
WITH rows AS (
  SELECT artifact_id, metadata
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.image-embedding-observation/v1'
), field_values AS (
  SELECT r.artifact_id, v.field_name, v.value, v.is_number
  FROM rows r
  CROSS JOIN LATERAL (VALUES
    ('embedding_model', r.metadata #>> '{observation,embedding_model}', false),
    ('embedding_dimension', r.metadata #>> '{observation,embedding_dimension}', true),
    ('frame_timestamp_seconds', r.metadata #>> '{observation,frame_timestamp_seconds}', true),
    ('review_state', r.metadata #>> '{observation,review_state}', false),
    ('source_truth_state', r.metadata ->> 'source_truth_state', false)
  ) AS v(field_name, value, is_number)
)
SELECT
  'image_embedding_metadata' AS entity,
  (SELECT count(*) FROM rows) AS matched_rows,
  field_name,
  count(value) AS non_null_count,
  round(100.0 * count(value) / NULLIF(count(*), 0), 1) AS presence_percent,
  count(DISTINCT value) AS distinct_count,
  min(CASE WHEN is_number AND value IS NOT NULL THEN value::numeric END) AS numeric_min,
  max(CASE WHEN is_number AND value IS NOT NULL THEN value::numeric END) AS numeric_max
FROM field_values
GROUP BY field_name
ORDER BY field_name;
COMMIT;
