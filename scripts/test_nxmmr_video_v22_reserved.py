#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Offline contracts for the one-time V2.2 reserved harness."""
from __future__ import annotations

import hashlib
import io
import json
import tempfile
import unittest
from dataclasses import asdict
from pathlib import Path
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
import sys
sys.path[:0] = [str(ROOT), str(ROOT / "scripts")]
import evaluate_nxmmr_video_v2_exact_runtime as common
import evaluate_nxmmr_video_v22_reserved as evaluator
import project_nxmmr_video_v22_reserved_oracle as projector
import run_nxmmr_video_v22_reserved as runner


class ReservedContracts(unittest.TestCase):
    def setUp(self):
        self.freeze_path = ROOT / "local-acceptance-models/nxmmr/private-benchmarks/raw-color-parity-20260831/video-v22-product-source-freeze.json"
        self.freeze = json.loads(self.freeze_path.read_text())
        self.split = json.loads((ROOT / "local-acceptance-models/nxmmr/private-benchmarks/reference-parity-adapter-v1/video-development-reserved-split.json").read_text())

    def test_named_digest_is_exact_and_canonical(self):
        digest = self.freeze.pop("freeze_digest")
        self.assertEqual(digest, evaluator.FREEZE_DIGEST)
        self.assertEqual(digest, runner.FREEZE_DIGEST)
        self.assertEqual(digest, common.canonical_digest(self.freeze))

    def test_frozen_sources_and_config_remain_exact(self):
        for name, digest in self.freeze["source_files"].items():
            self.assertEqual(common.hash_file(ROOT / name), digest, name)
        self.assertEqual(common.hash_file(ROOT / "configuration/nxmmr_video_anpr_product_baseline_v22.json"),
                         self.freeze["configuration_sha256"])

    def test_reserved_partition_is_locked_and_disjoint(self):
        value = dict(self.split)
        digest = value.pop("split_digest")
        self.assertEqual(common.canonical_digest(value), digest)
        self.assertEqual(value["reserved_event_count"], 8)
        self.assertFalse(set(value["reserved_event_ids"]) & set(value["development_event_ids"]))

    def test_reserved_schedule_never_enters_development_intervals(self):
        allowed = [common.TimeInterval(**item) for item in self.split["reserved_exclusion_intervals"]]
        prohibited = [common.TimeInterval(**item) for item in self.split["development_allowed_intervals"]]
        timestamps = common.schedule_base_frames(60.0, 60.0,
            common.FramePolicy("fixed-4fps-fastplate-vehicle-v2", 4.0, maximum_analyzed_frames=300),
            allowed_intervals=allowed)
        self.assertTrue(timestamps)
        self.assertFalse(any(interval.contains(round(t * 60) / 60)
                             for t in timestamps for interval in prohibited))

    def test_lexical_projection_never_decodes_unselected_payload(self):
        source = io.StringIO('event_id,value\nDEV,"secret,\nmultiline"\nRES,"allowed"\n')
        projected = "".join(projector.selected_records(source, {"RES"}))
        self.assertEqual(projected, 'event_id,value\nRES,"allowed"\n')
        self.assertNotIn("secret", projected)

    def test_projection_receipt_is_signed_and_one_time(self):
        value = {"reserved_evaluation_count": 1, "model_output_used_as_oracle": False}
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "receipt.json"
            projector.write_signed(path, value)
            stored = json.loads(path.read_text())
            digest = stored.pop("receipt_digest")
            self.assertEqual(projector.canonical_digest(stored), digest)
            with self.assertRaisesRegex(RuntimeError, "refusing to overwrite"):
                projector.write_signed(path, value)

    def test_runner_refuses_any_existing_consumption_marker(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            registration = {"authorization_sha256": runner.AUTHORIZATION_SHA256,
                "candidate_freeze_digest": runner.FREEZE_DIGEST,
                "reserved_evaluation_limit": 1}
            runner.write_signed(root / "registration.json", registration)
            (root / "consumption-start.json").write_text("already consumed")
            with patch.object(runner, "RESULTS", root), self.assertRaisesRegex(RuntimeError, "repeat forbidden"):
                runner.run()

    def test_selected_policy_and_semantics_are_frozen(self):
        self.assertEqual(asdict(common.SELECTED_POLICY), self.freeze["selected_policy"])
        config = json.loads((ROOT / "configuration/nxmmr_video_anpr_product_baseline_v22.json").read_text())
        self.assertEqual(config["ocr_input"]["selected_mode"], "RAW_RGB")
        self.assertFalse(config["frame_policy"]["interpolated_observations"])
        self.assertFalse(config["authority"]["persistent_tracking"])
        self.assertFalse(config["authority"]["vehicle_identity"])

    def test_scorer_sources_are_frozen(self):
        self.assertEqual(common.hash_file(ROOT / "scripts/benchmark_nexusai_nxmmr_anpr.py"),
                         "a65fdb599773cee766966ae24674db8435322130fc005218d0df1d046ab68830")
        self.assertEqual(common.hash_file(ROOT / "scripts/benchmark_nexusai_nxmmr_parity_adapter.py"),
                         "42b00af2259c127ff26bb05b181b300d03e8a251f9b8ea21ec6a6c38af2ce4ca")

    def test_authorization_hash_and_count_are_fixed(self):
        self.assertEqual(runner.AUTHORIZATION_SHA256,
                         "47be79708176cbb2caa1a7d9db0154e6f0ea0c0ad004b8c5d5273aa24f831626")
        self.assertEqual(self.freeze["reserved_evaluation_count"], 0)


if __name__ == "__main__":
    unittest.main(verbosity=2)
