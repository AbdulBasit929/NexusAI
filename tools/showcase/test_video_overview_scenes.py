"""Tests of the scene generator of the video overview (docs/data/video-overview-preregistration-20261125.md): the rules a scorer will rely on.

Run inside the worker or evaluation image (numpy and OpenCV are not on the host); no ffmpeg is needed:

    docker run --rm --network none --entrypoint python -v <repo>/tools/showcase:/tools:ro nexusai-media-eval:latest -B -m unittest discover -s /tools -p test_video_overview_scenes.py
"""
import os
import sys
import unittest

import numpy as np

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)

import build_video_overview_scenes as g  # noqa: E402


class Partition(unittest.TestCase):
    def test_the_split_is_fixed_and_the_sealed_part_is_25_scenes(self):
        dev = [i for i in g.SCENE_IDS if g.partition_of(i) == "dev"]
        self.assertEqual((len(dev), len(g.SCENE_IDS) - len(dev)), (95, 25))

    def test_the_rule_is_the_last_hex_digit_of_the_sha256_of_the_id(self):
        for i in g.SCENE_IDS[:30]:
            digit = g.sha_hex(i)[-1]
            self.assertEqual(g.partition_of(i), "dev" if digit in "0123456789ab" else "sealed")

    def test_sealed_scenes_are_refused_without_the_flag(self):
        sealed = next(i for i in g.SCENE_IDS if g.partition_of(i) == "sealed")
        self.assertEqual(g.main(["--ids", sealed, "--parts", "motion", "--out", "/nonexistent-out"]), 2)


class Determinism(unittest.TestCase):
    def test_the_same_id_gives_the_same_pictures_and_truth(self):
        a = g.motion_shot("mf1-0007", seconds=20)
        b = g.motion_shot("mf1-0007", seconds=20)
        self.assertEqual(g.motion_truth("mf1-0007", a), g.motion_truth("mf1-0007", b))
        for k in (0, 57, 140):
            self.assertTrue(np.array_equal(a.frame(k), b.frame(k)))

    def test_different_ids_differ(self):
        self.assertFalse(np.array_equal(g.motion_shot("mf1-0007", seconds=10).frame(3), g.motion_shot("mf1-0008", seconds=10).frame(3)))


class MotionTruth(unittest.TestCase):
    def test_intervals_are_apart_and_inside_the_scene(self):
        rng = g.rng_of("t", 1)
        for n in (1, 6, 12):
            ivs = g.plan_intervals(rng, 10, 1800, n)
            self.assertGreaterEqual(len(ivs), 1)
            for (s1, d1), (s2, d2) in zip(ivs, ivs[1:]):
                self.assertGreaterEqual((s2 - (s1 + d1)) / 10.0, g.MIN_GAP_SECONDS - 0.11)
            self.assertGreaterEqual(ivs[0][0], 0)
            self.assertLess(ivs[-1][0] + ivs[-1][1], 1800)
            for _, d in ivs:
                self.assertTrue(3 * 10 - 1 <= d <= 40 * 10 + 1)

    def test_one_scene_in_four_has_no_motion(self):
        none = [i for i in g.SCENE_IDS if g.seed_of(i, "none") % 4 == 0]
        self.assertTrue(20 <= len(none) <= 40)
        shot = g.motion_shot(none[0], seconds=20)
        self.assertEqual(g.motion_truth(none[0], shot)["n_intervals"], 0)

    def test_frames_inside_the_truth_interval_show_the_object_and_frames_outside_do_not(self):
        shot = g.motion_shot(next(i for i in g.SCENE_IDS if g.seed_of(i, "none") % 4 != 0), seconds=60)
        truth = g.motion_truth("x", shot)
        self.assertGreaterEqual(truth["n_intervals"], 1)
        passes = shot.passes

        def without_objects(k):
            shot.passes = []
            try:
                return shot.frame(k)
            finally:
                shot.passes = passes

        some = 0
        for o, p in zip(truth["objects"], passes):
            mid = (o["core_start_frame"] + o["core_end_frame"]) // 2
            changed = int((np.abs(shot.frame(mid).astype(np.int16) - without_objects(mid).astype(np.int16)).max(axis=2) > 8).sum())
            self.assertGreater(changed, 0.05 * p.w * p.h * p.fraction(mid), "an object must be drawn in the middle of its interval")
            # the first core frame has at least 25% of the box inside the picture, the frame before it has less
            self.assertGreaterEqual(p.fraction(o["core_start_frame"]), 0.25)
            self.assertLess(p.fraction(o["core_start_frame"] - 1), 0.25)
            self.assertGreaterEqual(p.fraction(o["core_end_frame"] - 1), 0.25)
            self.assertLess(p.fraction(o["core_end_frame"]), 0.25)
            some += 1
        self.assertGreater(some, 0)


class Sequence(unittest.TestCase):
    def test_every_sequence_has_8_hard_cuts_3_dissolves_and_2_fades_and_enough_hard_cuts_in_all(self):
        sealed = [i for i in g.SCENE_IDS if g.partition_of(i) == "sealed"]
        hard = 0
        for sid in sealed:
            plan = g.plan_sequence(sid)
            self.assertEqual(sorted(plan["transitions"]), sorted(g.TRANSITION_PLAN))
            self.assertEqual(sum(plan["lens"]), plan["total"])
            self.assertTrue(all(n >= int(g.MIN_SHOT_SECONDS * 10) for n in plan["lens"]))
            _, truth = g.sequence_effects(plan)
            hard += sum(1 for c in truth if c["type"] == "hard")
        self.assertGreaterEqual(hard, 150)

    def test_a_fade_and_a_dissolve_fit_inside_their_shots(self):
        for sid in g.SCENE_IDS[:20]:
            plan = g.plan_sequence(sid)
            fx, truth = g.sequence_effects(plan)
            self.assertTrue(all(0 <= i < plan["total"] for i in fx))
            for c in truth:
                if c["type"] != "hard":
                    self.assertTrue(0 <= c["start_frame"] and c["end_frame"] <= plan["total"])


class Clock(unittest.TestCase):
    def test_the_text_follows_the_model_and_wraps_at_24_hours(self):
        m = g.ClockModel("mf1-0003", 5400)
        t = 125
        s = int(np.floor(m.start_sod + t + m.offset(t))) % 86400
        self.assertEqual(m.text(t), "%02d:%02d:%02d" % (s // 3600, s // 60 % 60, s % 60))
        m.start_sod, m.init_offset, m.drift, m.jumps = 86395, 0, 0.0, []
        self.assertEqual(m.text(10), "00:00:05")

    def test_jumps_are_apart_inside_the_clip_and_at_least_10_seconds(self):
        seen = 0
        for sid in g.SCENE_IDS[:40]:
            m = g.ClockModel(sid, 5400)
            for a, b in zip(m.jumps, m.jumps[1:]):
                self.assertGreaterEqual(b["at_s"] - a["at_s"], 600)
            for j in m.jumps:
                self.assertTrue(600 <= j["at_s"] <= 5400 - 600 and abs(j["delta_s"]) >= 10)
                seen += 1
        self.assertGreater(seen, 10)

    def test_drift_is_plus_or_minus_2_seconds_per_hour_or_none(self):
        self.assertTrue({g.ClockModel(i, 5400).drift for i in g.SCENE_IDS[:40]} <= {0.0, 2.0, -2.0})

    def test_truth_lists_the_offset_at_every_full_minute(self):
        t = g.ClockModel("mf1-0003", 3600).truth(3600)
        self.assertEqual(len(t["minute_samples"]), 60)
        self.assertEqual([s["t_s"] for s in t["minute_samples"]][:3], [0, 60, 120])


if __name__ == "__main__":
    unittest.main()
