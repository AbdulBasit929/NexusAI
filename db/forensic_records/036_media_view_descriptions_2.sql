-- Migration 036: plainer descriptions of the media views, second round (comments only; no view, table or row changes).
--
-- Why. Measured 2026-11-12 on the second fresh set of free-form video questions (docs/data/video-questions-20261112.md): "Name the longest video" failed twice on a column that the inventory view does not have
-- (both views were offered; the video view lists every video and now says so), "on how many frames in the whole case was a plate read" counted reads and not frames, and "which videos contain spoken audio" was
-- answered from the audio-track flag. The model reads its documentation from the database's own comments, so the comments say it. Migration 035 is not changed (it is applied).
--
-- Scope. One COMMENT ON VIEW and three COMMENT ON COLUMN statements on views of migration 023. The pinned catalogue is regenerated with tools/case_generator/check_ask_catalogue.py --write-golden.
-- Rollback restores the texts of migrations 023 (ops/036_media_view_descriptions_2/rollback.sql). Idempotent.
BEGIN;
SET LOCAL lock_timeout = '10s';
COMMENT ON VIEW analyst.v_video_files IS
  'One row per video file of the case: its length in seconds (its running time or duration), its size in pixels and whether it has an audio track, as the container''s own header states them (empty when the container is not one the header reader knows). Use it, and not v_evidence_inventory, for anything about the videos themselves: how many there are, which is the longest, how long they are in total, which have sound.';
COMMENT ON COLUMN analyst.v_video_files.duration_seconds IS
  'Length of the video in seconds (its running time or duration), from the container header; empty when the header could not be read.';
COMMENT ON COLUMN analyst.v_video_files.has_audio IS
  'True when the container has an audio track, false when it has none, empty when the header could not be read. It does not say that anyone speaks: for what was said use v_speech_segments.';
COMMENT ON COLUMN analyst.v_video_plate_frames.frame_number IS
  'The number of the frame in the video, counted from 0. A frame can hold several reads: to count the frames that had a plate count distinct frame_number within one video (distinct evidence_id and frame_number across videos); to count the reads count the rows.';
COMMIT;
