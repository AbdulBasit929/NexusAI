-- Rollback for 036_media_view_descriptions_2.sql: restores the descriptions of migration 023. Comments only.
BEGIN;
COMMENT ON VIEW analyst.v_video_files IS 'One row per video file: its length in seconds, its size in pixels and whether it has an audio track, as the container''s own header states them (empty when the container is not one the header reader knows). Use it for how long a video is and how much footage the case holds.';
COMMENT ON COLUMN analyst.v_video_files.duration_seconds IS 'Length of the video in seconds, from the container header; empty when the header could not be read.';
COMMENT ON COLUMN analyst.v_video_files.has_audio IS 'True when the container has an audio track, false when it has none, empty when the header could not be read.';
COMMENT ON COLUMN analyst.v_video_plate_frames.frame_number IS 'The number of the frame in the video, counted from 0.';
COMMIT;
