-- Rollback for 035_media_view_descriptions.sql: restores the descriptions of migration 023. Comments only.
BEGIN;
COMMENT ON COLUMN analyst.v_plate_reads.first_seen_seconds IS 'For a video, the second the plate was first seen; empty for a photo.';
COMMENT ON COLUMN analyst.v_plate_reads.last_seen_seconds IS 'For a video, the second the plate was last seen; empty for a photo.';
COMMENT ON COLUMN analyst.v_plate_reads.source IS 'image for one sighting in a photo, video for a group of the sightings of the same plate in one video.';
COMMENT ON COLUMN analyst.v_plate_reads.needs_review IS 'True: a model candidate that a person must check (always true); never an identification of a vehicle owner.';
COMMENT ON COLUMN analyst.v_video_plate_frames.seen_at_seconds IS 'The second of the video the frame was read at.';
COMMENT ON COLUMN analyst.v_video_plate_frames.needs_review IS 'True: a model candidate that a person must check (always true).';
COMMENT ON COLUMN analyst.v_ocr_regions.needs_review IS 'True: a person must check the reading before it is relied on (always true for a machine reading).';
COMMENT ON COLUMN analyst.v_ocr_evidence_summary.needs_review IS 'True: a person must check the reading before it is relied on.';
COMMENT ON COLUMN analyst.v_speech_segments.needs_review IS 'True: a person must listen and check before the text is relied on (always true).';
COMMENT ON COLUMN analyst.v_speech_segments.start_seconds IS 'Start of the segment in seconds from the start of the recording.';
COMMENT ON COLUMN analyst.v_speech_segments.end_seconds IS 'End of the segment in seconds from the start of the recording.';
COMMIT;
