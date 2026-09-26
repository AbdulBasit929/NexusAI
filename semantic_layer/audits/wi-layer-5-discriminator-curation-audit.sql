-- WI-LAYER-5 independent read-only oracle for discriminator-backed image
-- entities and the source-modality ruling for the three audio contracts.

BEGIN READ ONLY;

-- Every curated image field is counted inside its own exact partition. This is
-- the acceptance table for the two new entities, not a production-loader query.
WITH image_rows AS (
  SELECT metadata ->> 'observation_type' AS observation_type, metadata
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.image-observation/v1'
), entity_fields(entity, observation_type, field_name, path) AS (
  VALUES
    ('image_technical_metadata', 'image_technical_observation', 'format', ARRAY['observation','source_metadata','format']),
    ('image_technical_metadata', 'image_technical_observation', 'color_model', ARRAY['observation','source_metadata','color_model']),
    ('image_technical_metadata', 'image_technical_observation', 'width_pixels', ARRAY['observation','source_metadata','width_pixels']),
    ('image_technical_metadata', 'image_technical_observation', 'height_pixels', ARRAY['observation','source_metadata','height_pixels']),
    ('image_technical_metadata', 'image_technical_observation', 'display_width_pixels', ARRAY['observation','source_metadata','display_width_pixels']),
    ('image_technical_metadata', 'image_technical_observation', 'display_height_pixels', ARRAY['observation','source_metadata','display_height_pixels']),
    ('image_technical_metadata', 'image_technical_observation', 'exif_orientation', ARRAY['observation','source_metadata','exif_orientation']),
    ('image_technical_metadata', 'image_technical_observation', 'exif_orientation_label', ARRAY['observation','source_metadata','exif_orientation_label']),
    ('image_technical_metadata', 'image_technical_observation', 'source_size_bytes', ARRAY['observation','source_metadata','source_size_bytes']),
    ('image_technical_metadata', 'image_technical_observation', 'pixel_count', ARRAY['observation','source_metadata','pixel_count']),
    ('image_technical_metadata', 'image_technical_observation', 'container_boundary_validated', ARRAY['observation','source_metadata','container_boundary_validated']),
    ('image_technical_metadata', 'image_technical_observation', 'extension_content_match', ARRAY['observation','source_metadata','extension_content_match']),
    ('image_technical_metadata', 'image_technical_observation', 'downstream_decode_allowed', ARRAY['observation','source_metadata','downstream_decode_allowed']),
    ('image_technical_metadata', 'image_technical_observation', 'decode_review_required', ARRAY['observation','source_metadata','decode_review_required']),
    ('image_technical_metadata', 'image_technical_observation', 'validation_state', ARRAY['observation','source_metadata','validation_state']),
    ('image_technical_metadata', 'image_technical_observation', 'inspection_mode', ARRAY['observation','source_metadata','inspection_mode']),
    ('image_technical_metadata', 'image_technical_observation', 'metadata_privacy_policy', ARRAY['observation','source_metadata','metadata_privacy_policy']),
    ('image_technical_metadata', 'image_technical_observation', 'source_truth_state', ARRAY['source_truth_state']),
    ('video_sampled_frame', 'video_sampled_frame_observation', 'frame_number', ARRAY['observation','frame_number']),
    ('video_sampled_frame', 'video_sampled_frame_observation', 'frame_number_semantics', ARRAY['observation','frame_number_semantics']),
    ('video_sampled_frame', 'video_sampled_frame_observation', 'sampling_interval_seconds', ARRAY['observation','sampling_interval_seconds']),
    ('video_sampled_frame', 'video_sampled_frame_observation', 'sampling_strategy', ARRAY['observation','sampling_strategy']),
    ('video_sampled_frame', 'video_sampled_frame_observation', 'storage_state', ARRAY['observation','storage_state']),
    ('video_sampled_frame', 'video_sampled_frame_observation', 'source_truth_state', ARRAY['source_truth_state'])
), measured AS (
  SELECT
    f.entity,
    f.observation_type,
    f.field_name,
    count(*) AS partition_rows,
    count(r.metadata #>> f.path) AS present_rows,
    count(DISTINCT r.metadata #>> f.path) AS distinct_values
  FROM entity_fields f
  JOIN image_rows r USING (observation_type)
  GROUP BY f.entity, f.observation_type, f.field_name
)
SELECT
  entity,
  observation_type,
  field_name,
  present_rows,
  partition_rows,
  round(100.0 * present_rows / NULLIF(partition_rows, 0), 1) AS presence_percent,
  distinct_values
FROM measured
ORDER BY entity, field_name;

-- Cross-partition view: this makes the reason for the split visible. Technical
-- dimensions/checks are 20/20 and 0/6; frame fields are 0/20 and 6/6. The four
-- EXIF/display fields are the deliberate 8/20 exception within the technical side.
WITH image_rows AS (
  SELECT metadata ->> 'observation_type' AS observation_type, metadata
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.image-observation/v1'
), fields(field_name, path) AS (
  VALUES
    ('format', ARRAY['observation','source_metadata','format']),
    ('width_pixels', ARRAY['observation','source_metadata','width_pixels']),
    ('display_width_pixels', ARRAY['observation','source_metadata','display_width_pixels']),
    ('exif_orientation', ARRAY['observation','source_metadata','exif_orientation']),
    ('frame_number', ARRAY['observation','frame_number']),
    ('sampling_interval_seconds', ARRAY['observation','sampling_interval_seconds']),
    ('sampling_strategy', ARRAY['observation','sampling_strategy']),
    ('storage_state', ARRAY['observation','storage_state'])
)
SELECT
  f.field_name,
  count(r.metadata #>> f.path) FILTER (WHERE r.observation_type = 'image_technical_observation') AS technical_present_of_20,
  count(r.metadata #>> f.path) FILTER (WHERE r.observation_type = 'video_sampled_frame_observation') AS frame_present_of_6
FROM fields f
CROSS JOIN image_rows r
GROUP BY f.field_name
ORDER BY f.field_name;

-- Audio observation-type population and exact source-modality coverage.
SELECT
  artifact_type,
  metadata ->> 'observation_type' AS observation_type,
  count(*) AS row_count
FROM forensic.derived_artifacts
WHERE collection_id = 'nexusai-multimodal-product-acceptance'
  AND artifact_type IN (
    'forensics.audio-timestamp-segment/v1',
    'forensics.audio-roman-urdu-segment/v1',
    'forensics.audio-observation/v1'
  )
GROUP BY artifact_type, metadata ->> 'observation_type'
ORDER BY artifact_type, observation_type;

-- Establish whether observation_type is literally the only distinguishing
-- field. It is not: storage_state is embedded-only for all three contracts;
-- timestamp timing_status is sparse; and the technical source_metadata object
-- has materially different standalone-WAV and video-container keys.
WITH audio_rows AS (
  SELECT artifact_type, metadata ->> 'observation_type' AS observation_type,
         key, value
  FROM forensic.derived_artifacts d
  CROSS JOIN LATERAL jsonb_each(d.metadata -> 'observation') AS e(key, value)
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type IN (
      'forensics.audio-timestamp-segment/v1',
      'forensics.audio-roman-urdu-segment/v1',
      'forensics.audio-observation/v1'
    )
), key_presence AS (
  SELECT artifact_type, observation_type, key,
         count(*) FILTER (WHERE value <> 'null'::jsonb) AS non_null_rows
  FROM audio_rows
  GROUP BY artifact_type, observation_type, key
)
SELECT artifact_type, observation_type, key, non_null_rows
FROM key_presence
ORDER BY artifact_type, key, observation_type;

-- The nested technical metadata is where the third contract departs most: the
-- four standalone rows expose audio-stream scalars, while the one embedded row
-- exposes video-container scalars. Arrays remain inventory evidence, not fields.
WITH technical_rows AS (
  SELECT metadata ->> 'observation_type' AS observation_type,
         metadata #> '{observation,source_metadata}' AS source_metadata
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.audio-observation/v1'
), technical_keys AS (
  SELECT observation_type, key, jsonb_typeof(value) AS value_type, value
  FROM technical_rows
  CROSS JOIN LATERAL jsonb_each(source_metadata)
)
SELECT observation_type, key, value_type,
       count(*) FILTER (WHERE value <> 'null'::jsonb) AS non_null_rows
FROM technical_keys
GROUP BY observation_type, key, value_type
ORDER BY key, observation_type;

COMMIT;
