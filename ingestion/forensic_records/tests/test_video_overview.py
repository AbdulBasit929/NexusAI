from __future__ import annotations

import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

import numpy as np

from ingestion.forensic_records.video_overview import (
    MotionParameters, compare_signatures, export_clip, file_sha256, intervals_from_series, motion_index, probe_video, shot_cuts, shot_cuts_window, video_signature)

HAVE_FFMPEG = shutil.which("ffmpeg") is not None and shutil.which("ffprobe") is not None


def make_clip(path: Path, seconds: float = 6.0, fps: int = 10, moving=((2.0, 3.5),), cut_at: float | None = None) -> None:
    """A small clip made with ffmpeg's lavfi sources: a static noisy grey picture with a white square sliding across it during each (start, end) interval; optionally a hard change of picture at cut_at."""
    import cv2

    width, height = 320, 180
    rng = np.random.default_rng(5)
    command = ["ffmpeg", "-v", "error", "-y", "-f", "rawvideo", "-pix_fmt", "bgr24", "-s", f"{width}x{height}", "-r", str(fps), "-i", "-", "-an", "-c:v", "libx264", "-pix_fmt", "yuv420p", str(path)]
    process = subprocess.Popen(command, stdin=subprocess.PIPE)
    base = np.full((height, width, 3), 90, np.uint8)
    for (x0, y0, x1, y1, grey) in ((10, 10, 120, 70, 170), (200, 30, 300, 150, 40), (30, 110, 140, 170, 130), (150, 90, 190, 120, 220)):   # structure that is not mirror symmetric
        cv2.rectangle(base, (x0, y0), (x1, y1), (grey, grey, grey), -1)
    other = np.full((height, width, 3), 190, np.uint8)
    cv2.circle(other, (80, 60), 40, (30, 30, 30), -1)
    for i in range(int(seconds * fps)):
        t = i / fps
        frame = (other if cut_at is not None and t >= cut_at else base).copy()
        for start, end in moving:
            if start <= t < end:
                x = int(10 + (width - 60) * (t - start) / (end - start))
                cv2.rectangle(frame, (x, 80), (x + 40, 120), (255, 255, 255), -1)
        noisy = np.clip(frame.astype(np.int16) + rng.integers(-2, 3, frame.shape), 0, 255).astype(np.uint8)
        process.stdin.write(noisy.tobytes())
    process.stdin.close()
    process.wait()


class IntervalTests(unittest.TestCase):
    def test_active_steps_become_intervals_with_gaps_closed_short_ones_dropped_and_padding(self):
        p = MotionParameters(analysis_fps=5.0, close_gap_seconds=1.0, minimum_seconds=1.0, pad_seconds=0.2, changed_fraction=0.01)
        series = np.zeros(100, np.float32)
        series[10:20] = 0.05            # steps 10..19: 2.0 s to 4.0 s
        series[22:26] = 0.05            # 0.4 s after the first run (gap under 1 s): the same interval, now 2.0 s to 5.2 s
        series[60:61] = 0.05            # one step: 0.2 s long, dropped
        series[80:90] = 0.05            # 16.0 s to 18.0 s
        out = intervals_from_series(series, p)
        self.assertEqual(len(out), 2)
        self.assertAlmostEqual(out[0]["start_seconds"], 1.8, places=2)
        self.assertAlmostEqual(out[0]["end_seconds"], 5.4, places=2)
        self.assertAlmostEqual(out[1]["start_seconds"], 15.8, places=2)

    def test_a_quiet_series_gives_no_intervals_and_an_empty_one_does_not_fail(self):
        self.assertEqual(intervals_from_series(np.zeros(50, np.float32)), [])
        self.assertEqual(intervals_from_series(np.zeros(0, np.float32)), [])


@unittest.skipUnless(HAVE_FFMPEG, "ffmpeg and ffprobe are needed")
class FileTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.dir = Path(self.directory.name)

    def tearDown(self):
        self.directory.cleanup()

    def test_probe_reads_a_good_file_exactly_and_says_why_a_bad_one_is_unreadable(self):
        good = self.dir / "good.mp4"
        make_clip(good, seconds=6.0, fps=10)
        info = probe_video(good)
        self.assertTrue(info["readable"])
        self.assertEqual((info["width"], info["height"], info["frames"], info["has_audio"]), (320, 180, 60, False))
        self.assertAlmostEqual(info["duration_seconds"], 6.0, places=3)
        self.assertAlmostEqual(info["fps"], 10.0, places=3)
        empty = self.dir / "empty.mp4"
        empty.write_bytes(b"")
        self.assertEqual((probe_video(empty)["readable"], "empty" in probe_video(empty)["reason"]), (False, True))
        noise = self.dir / "noise.mkv"
        noise.write_bytes(bytes(range(256)) * 20)
        bad = probe_video(noise)
        self.assertFalse(bad["readable"])
        self.assertTrue(bad["reason"])
        truncated = self.dir / "cut.mp4"
        data = good.read_bytes()
        truncated.write_bytes(data[: len(data) // 3])
        self.assertFalse(probe_video(truncated)["readable"] and probe_video(truncated).get("frames") == 60)
        self.assertEqual(probe_video(self.dir / "missing.mp4")["readable"], False)

    def test_the_motion_index_finds_the_moving_square_and_nothing_in_a_quiet_clip(self):
        moving = self.dir / "moving.mp4"
        make_clip(moving, seconds=10.0, moving=((2.0, 3.5), (6.5, 8.5)))       # more than the 2 s the index closes between two intervals
        index = motion_index(moving)
        self.assertEqual(index["interval_count"], 2)
        first, second = index["intervals"]
        self.assertLess(abs(first["start_seconds"] - 1.8), 0.6)
        self.assertLess(abs(second["end_seconds"] - 8.7), 0.6)
        quiet = self.dir / "quiet.mp4"
        make_clip(quiet, seconds=8.0, moving=())
        self.assertEqual(motion_index(quiet)["interval_count"], 0)

    def reencode(self, src: Path, name: str, vf: str | None = None, bitrate: str = "120k") -> Path:
        dst = self.dir / name
        cmd = ["ffmpeg", "-v", "error", "-y", "-i", str(src)] + (["-vf", vf] if vf else []) + ["-an", "-c:v", "libx264", "-b:v", bitrate, "-pix_fmt", "yuv420p", str(dst)]
        subprocess.run(cmd, check=True)
        return dst

    def test_the_signature_matches_a_reencode_a_mirror_and_a_crop_and_not_other_footage(self):
        original = self.dir / "original.mp4"
        make_clip(original, seconds=40.0, moving=((5.0, 9.0), (22.0, 30.0)))
        a = video_signature(original)
        self.assertGreaterEqual(a["seconds"], 39)
        low = compare_signatures(a, video_signature(self.reencode(original, "low.mp4")))
        self.assertTrue(low["same"], low)
        mirror = compare_signatures(a, video_signature(self.reencode(original, "mirror.mp4", "hflip")))
        self.assertTrue(mirror["same"] and mirror["mirrored"], mirror)
        crop = compare_signatures(a, video_signature(self.reencode(original, "crop.mp4", "crop=iw*0.95:ih*0.95,scale=320:180")))
        self.assertTrue(crop["same"], crop)
        trim = compare_signatures(a, video_signature(self.reencode(original, "trim.mp4", "trim=start=8:duration=25,setpts=PTS-STARTPTS")))
        self.assertTrue(trim["same"] and trim["shift_seconds"] == 8, trim)
        # the same camera view recorded again with other events: the background matches, the motion does not
        other_events = self.dir / "other_events.mp4"
        make_clip(other_events, seconds=40.0, moving=((12.0, 15.0), (33.0, 37.0)))
        self.assertFalse(compare_signatures(a, video_signature(other_events))["same"])
        quiet = self.dir / "quiet.mp4"
        make_clip(quiet, seconds=40.0, moving=())
        self.assertFalse(compare_signatures(a, video_signature(quiet))["same"])
        self.assertTrue(compare_signatures(video_signature(quiet), video_signature(self.reencode(quiet, "quiet_low.mp4")))["same"])

    def test_an_export_decodes_has_the_requested_frames_and_a_manifest_that_verifies(self):
        source = self.dir / "source.mp4"
        make_clip(source, seconds=8.0, moving=((2.0, 4.0),))
        before = file_sha256(source)
        manifest = export_clip(source, 2.0, 5.0, self.dir / "excerpt.mp4")
        self.assertTrue(manifest["decodes"] and manifest["frame_count_matches"], manifest)
        self.assertEqual(manifest["expected_frames"], 30)
        self.assertEqual(manifest["source_sha256"], before)
        self.assertEqual(manifest["output_sha256"], file_sha256(self.dir / "excerpt.mp4"))
        self.assertEqual(file_sha256(source), before)                       # the source is only read
        with self.assertRaises(ValueError):
            export_clip(source, 5.0, 2.0, self.dir / "bad.mp4")

    def test_the_persistent_change_detector_finds_the_cut_and_ignores_a_lamp_and_a_passing_square(self):
        cut = self.dir / "cut_window.mp4"
        make_clip(cut, seconds=8.0, moving=((1.0, 2.5),), cut_at=5.0)       # a square crosses at 1 to 2.5 s; the picture changes for good at 5 s
        cuts = shot_cuts_window(cut)["cuts"]
        self.assertEqual(len(cuts), 1, cuts)
        self.assertAlmostEqual(cuts[0]["seconds"], 5.0, delta=0.11)
        quiet = self.dir / "square_only.mp4"
        make_clip(quiet, seconds=8.0, moving=((1.0, 2.5), (4.0, 6.0)))
        self.assertEqual(shot_cuts_window(quiet)["cuts"], [])

    def test_overview_packets_carry_the_reading_contract_and_stable_ids(self):
        from ingestion.forensic_records.video_overview import VIDEO_OVERVIEW_CONTRACT, overview_packets
        source = self.dir / "packets.mp4"
        make_clip(source, seconds=10.0, moving=((2.0, 4.0),), cut_at=7.0)
        context = {"source_sha256": "b" * 64, "source_file": "packets.mp4", "evidence_id": "e-1", "version_id": "v-1"}
        packets, summary = overview_packets(source, context)
        again, _ = overview_packets(source, context)
        self.assertEqual([p["observation_id"] for p in packets], [p["observation_id"] for p in again])
        self.assertEqual(len({p["observation_id"] for p in packets}), len(packets))
        kinds = sorted({p["observation_type"] for p in packets})
        self.assertEqual(kinds, ["video_motion_interval_observation", "video_overview_probe_observation", "video_shot_cut_observation"])
        for p in packets:
            self.assertEqual(p["contract_version"], VIDEO_OVERVIEW_CONTRACT)
            rc = p["payload"]["reading_contract"]
            self.assertEqual((rc["tier"], rc["input_sha256"], rc["candidate"], rc["measured_error"]), ("UNMEASURED", "b" * 64, True, None))
            self.assertTrue(p["payload"]["manual_review_required"])
        self.assertEqual(summary["motion_intervals"], sum(1 for p in packets if p["observation_type"] == "video_motion_interval_observation"))
        self.assertGreaterEqual(summary["shot_cuts"], 1)

    def test_the_pipeline_stores_overview_observations_only_when_the_switch_is_on(self):
        from unittest import mock

        from ingestion.forensic_records import media_pipeline as mp

        source = self.dir / "pipeline.mp4"
        make_clip(source, seconds=10.0, moving=((2.0, 4.0),), cut_at=7.0)
        context = dict(evidence_id="e-1", version_id="v-1", source_sha256="c" * 64, source_file="pipeline.mp4", admitted_metadata={})

        def run(switch):
            with mock.patch.object(mp, "_extract_video_audio", lambda _path: None), mock.patch.object(mp, "_sample_video_frames", lambda _path, **_options: []),                  mock.patch.dict("os.environ", {"FORENSIC_VIDEO_OVERVIEW": switch}):
                return mp.DeterministicMediaProcessor().process(source, modality="video", **context)

        off = run("false")
        self.assertEqual(off.metadata["result_states"]["video_overview"], "NOT_RUN")
        self.assertFalse([o for o in off.observations if o.contract_version == "forensics.video-overview/v1"])
        on = run("true")
        self.assertEqual(on.metadata["result_states"]["video_overview"], "COMPLETE_RESULTS")
        overview = [o for o in on.observations if o.contract_version == "forensics.video-overview/v1"]
        self.assertTrue(any(o.observation_type == "video_motion_interval_observation" for o in overview))
        self.assertTrue(any(o.observation_type == "video_shot_cut_observation" for o in overview))
        self.assertEqual(len(on.observations) - len(overview), len(off.observations))      # everything else is untouched

    def test_a_cut_is_found_at_the_new_pictures_first_frame(self):
        cut = self.dir / "cut.mp4"
        make_clip(cut, seconds=6.0, moving=(), cut_at=3.0)
        cuts = shot_cuts(cut)["cuts"]
        self.assertEqual(len(cuts), 1)
        self.assertAlmostEqual(cuts[0]["seconds"], 3.0, delta=0.11)


if __name__ == "__main__":
    unittest.main()
