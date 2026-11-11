from __future__ import annotations

import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace

import cv2
import numpy as np

from ingestion.forensic_records.video_anpr_nexus_v4 import (
    ADAPTER_ID,
    PlateObservationV4,
    VideoANPRNexusV4Processor,
    combined_quality,
    sharpness_score,
)
from ingestion.forensic_records.video_anpr_parity_adapter import PlateBounds

FPS = 25
SECONDS = 6


def write_video(path: Path) -> None:
    """A small real video: frame i has i in its first pixel, so a fake engine can tell the frames apart; noise keeps the codec honest."""
    writer = cv2.VideoWriter(str(path), cv2.VideoWriter_fourcc(*"mp4v"), FPS, (320, 180))
    rng = np.random.default_rng(1)
    for i in range(FPS * SECONDS):
        frame = rng.integers(60, 120, (180, 320, 3), dtype=np.uint8)
        frame[:8, :8] = 255 - (i % 200)          # survives lossy coding well enough to be read back as a frame marker (only used as a hint)
        writer.write(frame)
    writer.release()


class ScriptedEngine:
    """Returns the reads scripted for a time window, whatever the frame; the processor tells frames apart by call order through `calls`."""

    def __init__(self, reads_by_call, box=(100, 60, 200, 90)):
        self.reads_by_call = reads_by_call
        self.box = box
        self.calls = 0

    def predict(self, _frame):
        reads = self.reads_by_call(self.calls)
        self.calls += 1
        return [SimpleNamespace(detection=SimpleNamespace(confidence=0.9, bounding_box=SimpleNamespace(x1=self.box[0], y1=self.box[1], x2=self.box[2], y2=self.box[3])),
                                ocr=SimpleNamespace(text=text, confidence=[confidence] * max(1, len(text)))) for text, confidence in reads]


def processor_for(engine, **options):
    image_processor = SimpleNamespace(engine=engine, model_hashes={"detector": "d" * 64, "ocr": "o" * 64, "ocr_config": "c" * 64},
                                      processor_id="nexus-plates-onnx-cpu", detector_id="rfdetr-alpr-test", ocr_id="cct-test")
    return VideoANPRNexusV4Processor(image_processor, cv2, **options)


CONTEXT = {"source_sha256": "a" * 64, "source_file": "clip.mp4", "evidence_id": "e-1", "version_id": "v-1"}


class VideoANPRNexusV4Tests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.path = Path(self.directory.name) / "clip.mp4"
        write_video(self.path)

    def tearDown(self):
        self.directory.cleanup()

    def test_the_selected_string_is_one_that_was_read_and_the_group_says_it_is_not_tracking(self):
        # a plate read as ABC1234 on most frames and as ABC1284 on a few: the group selects what was read, never a blend
        processor = processor_for(ScriptedEngine(lambda n: [("ABC-1234", 0.95)] if n % 5 else [("ABC-1284", 0.6)]), refinement=False)
        run = processor.run_video(self.path, context=dict(CONTEXT))
        self.assertEqual(len(run.groups), 1)
        group = run.groups[0]
        self.assertEqual(group.selected_normalized_plate, "ABC1234")
        self.assertIn(group.selected_normalized_plate, {o.normalized_ocr for o in run.observations})
        metadata = run.result.metadata
        self.assertFalse(metadata["persistent_tracking"])
        self.assertFalse(metadata["interpolated_observations"])
        self.assertEqual(metadata["result_states"]["video_anpr"], "COMPLETE_RESULTS")
        packets = [o for o in run.result.observations if o.observation_type == "video_anpr_plate_group_observation"]
        self.assertEqual(len(packets), 1)
        self.assertFalse(packets[0].payload["persistent_tracking"])

    def test_every_observation_carries_the_engines_own_hashes_and_ids(self):
        run = processor_for(ScriptedEngine(lambda n: [("XYZ9876", 0.9)]), refinement=False).run_video(self.path, context=dict(CONTEXT))
        packets = [o for o in run.result.observations if o.observation_type == "anpr_ocr_observation"]
        self.assertTrue(packets)
        for packet in packets:
            self.assertEqual(packet.payload["detector_model_sha256"], "d" * 64)
            self.assertEqual(packet.payload["ocr_model_sha256"], "o" * 64)
            self.assertEqual(packet.payload["processor"], ADAPTER_ID)
            self.assertEqual(packet.payload["engine_detector"], "rfdetr-alpr-test")
            self.assertTrue(packet.payload["manual_review_required"])
            self.assertFalse(packet.payload["interpolated"])

    def test_refinement_reads_more_frames_around_a_read_and_stays_bounded(self):
        engine = ScriptedEngine(lambda n: [("LEA1234", 0.9)])
        base = processor_for(engine, refinement=False).run_video(self.path, context=dict(CONTEXT))
        refined = processor_for(ScriptedEngine(lambda n: [("LEA1234", 0.9)]), refinement=True).run_video(self.path, context=dict(CONTEXT))
        self.assertEqual(base.frames_refined, 0)
        self.assertGreater(refined.frames_refined, 0)
        self.assertGreater(refined.coverage.frames_decoded, base.coverage.frames_decoded)
        capped = processor_for(ScriptedEngine(lambda n: [("LEA1234", 0.9)]), refinement=True, maximum_frames=10).run_video(self.path, context=dict(CONTEXT))
        self.assertLessEqual(capped.coverage.frames_decoded, 10)
        self.assertEqual(len({o.frame_number for o in refined.observations}), refined.coverage.frames_decoded)   # no frame read twice

    def test_no_reads_is_a_stated_zero_result_not_an_error(self):
        run = processor_for(ScriptedEngine(lambda n: []), refinement=True).run_video(self.path, context=dict(CONTEXT))
        self.assertEqual(run.groups, ())
        self.assertEqual(run.result.metadata["result_states"]["video_anpr"], "COMPLETE_ZERO_RESULTS")
        self.assertEqual(run.frames_refined, 0)

    def test_an_unreadable_source_is_refused(self):
        bad = Path(self.directory.name) / "not-a-video.mp4"
        bad.write_bytes(b"not a video")
        with self.assertRaises(RuntimeError):
            processor_for(ScriptedEngine(lambda n: [])).run_video(bad, context=dict(CONTEXT))

    def test_a_crisp_crop_scores_above_a_blurred_one_of_the_same_size(self):
        rng = np.random.default_rng(3)
        crisp = np.zeros((40, 120, 3), np.uint8)
        for x in range(0, 120, 6):
            crisp[:, x:x + 3] = 255
        crisp = np.clip(crisp.astype(int) + rng.integers(-5, 5, crisp.shape), 0, 255).astype(np.uint8)
        blurred = cv2.GaussianBlur(crisp, (0, 0), 6)
        self.assertGreater(sharpness_score(cv2, crisp), 0.5)
        self.assertLess(sharpness_score(cv2, blurred), 0.2)
        self.assertGreater(combined_quality(0.5, sharpness_score(cv2, crisp)), combined_quality(0.5, sharpness_score(cv2, blurred)))

    def test_with_equal_counts_the_sharper_reads_win_the_group(self):
        def observation(text, number, quality):
            return PlateObservationV4(observation_id=f"{text}-{number}", source_sha256="a" * 64, source_file="clip.mp4", frame_number=number, timestamp_seconds=number / 25,
                                      source_frame_sha256="f" * 64, bounds=PlateBounds(100, 60, 100, 30), raw_ocr=text, normalized_ocr=text, detector_confidence=0.9,
                                      ocr_confidence=0.9, crop_sha256="c" * 64, detector_model_sha256="d" * 64, ocr_model_sha256="o" * 64, crop_quality=quality, sharpness=quality)
        from ingestion.forensic_records.video_anpr_nexus_v4 import DEFAULT_POLICY
        from ingestion.forensic_records.video_anpr_product_v2 import aggregate_product_observations
        reads = [observation("ABC1234", 0, 0.9), observation("ABC1234", 6, 0.9), observation("ABC1234", 12, 0.9),
                 observation("ABC1284", 3, 0.2), observation("ABC1284", 9, 0.2), observation("ABC1284", 15, 0.2)]
        groups = aggregate_product_observations(reads, DEFAULT_POLICY)
        self.assertEqual([g.selected_normalized_plate for g in groups], ["ABC1234"])
        self.assertEqual(sorted(a.normalized_plate_text for a in groups[0].alternatives), ["ABC1234", "ABC1284"])


    def test_groups_of_the_same_text_that_touch_in_time_are_merged_and_others_are_not(self):
        from ingestion.forensic_records.video_anpr_nexus_v4 import DEFAULT_POLICY, merge_same_text_groups
        from ingestion.forensic_records.video_anpr_product_v2 import aggregate_product_observations

        def observation(text, number, x):
            return PlateObservationV4(observation_id=f"{text}-{number}", source_sha256="a" * 64, source_file="clip.mp4", frame_number=number, timestamp_seconds=number / 25,
                                      source_frame_sha256="f" * 64, bounds=PlateBounds(x, 60, 40, 12), raw_ocr=text, normalized_ocr=text, detector_confidence=0.9,
                                      ocr_confidence=0.9, crop_sha256="c" * 64, detector_model_sha256="d" * 64, ocr_model_sha256="o" * 64, crop_quality=0.5, sharpness=0.5)
        # one plate read three times at one place and three times at a far place a moment later (the grouping cannot associate them), another plate much later
        reads = [observation("ABC1234", n, 10) for n in (0, 5, 10)] + [observation("ABC1234", n, 900) for n in (15, 20, 25)] + [observation("XYZ9876", n, 10) for n in (200, 205, 210)]
        groups = aggregate_product_observations(reads, DEFAULT_POLICY)
        self.assertEqual(sorted(g.selected_normalized_plate for g in groups), ["ABC1234", "ABC1234", "XYZ9876"])
        merged = merge_same_text_groups(groups, reads, gap_seconds=1.5)
        self.assertEqual(sorted((g.selected_normalized_plate, g.support_count) for g in merged), [("ABC1234", 6), ("XYZ9876", 3)])
        self.assertEqual(len(next(g for g in merged if g.selected_normalized_plate == "ABC1234").observation_ids), 6)
        apart = merge_same_text_groups(groups, reads, gap_seconds=0.01)
        self.assertEqual(len(apart), 3)


    def test_two_engines_read_every_frame_keep_their_own_hashes_and_reinforce_each_other(self):
        def engine_for(text, box):
            return ScriptedEngine(lambda n: [(text, 0.9)], box)       # real engines draw slightly different boxes; identical boxes of one frame are one observation to the grouping
        primary = SimpleNamespace(engine=engine_for("ABC1234", (100, 60, 200, 90)), model_hashes={"detector": "a" * 64, "ocr": "b" * 64}, processor_id="engine-one", detector_id="d1", ocr_id="o1")
        second = SimpleNamespace(engine=engine_for("ABC1284", (104, 62, 206, 92)), model_hashes={"detector": "c" * 64, "ocr": "d" * 64}, processor_id="engine-two", detector_id="d2", ocr_id="o2")
        run = VideoANPRNexusV4Processor(primary, cv2, extra_processors=(second,), refinement=False).run_video(self.path, context=dict(CONTEXT))
        self.assertEqual(primary.engine.calls, second.engine.calls)       # both engines read every analysed frame
        packets = [o for o in run.result.observations if o.observation_type == "anpr_ocr_observation"]
        by_engine = {p.payload["engine_processor"]: p.payload["detector_model_sha256"] for p in packets}
        self.assertEqual(by_engine, {"engine-one": "a" * 64, "engine-two": "c" * 64})
        self.assertEqual(run.result.metadata["selected_policy"]["strategy"], "confidence_weighted_support")
        self.assertEqual(len(run.groups), 1)                              # the two reads are one edit apart and close in space: one group that keeps both as alternatives
        self.assertEqual({a.normalized_plate_text for a in run.groups[0].alternatives}, {"ABC1234", "ABC1284"})


if __name__ == "__main__":
    unittest.main()
