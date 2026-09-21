#!/usr/bin/env python3
"""Focused tests for the local-only NX-MMR human verification workflow."""

from __future__ import annotations

import csv
import hashlib
import json
import sys
import tempfile
import unittest
from pathlib import Path


sys.path.insert(0, str(Path(__file__).parent))
from nxmmr_human_verification import PRIVATE_RELATIVE, ReviewStore  # noqa: E402


def digest(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


class HumanVerificationTests(unittest.TestCase):
    def make_store(self, root: Path) -> ReviewStore:
        image_root = root / "private-images"
        image_root.mkdir()
        private_root = root / PRIVATE_RELATIVE
        images_dir = private_root / "images"
        video_dir = private_root / "video"
        images_dir.mkdir(parents=True)
        video_dir.mkdir(parents=True)
        fields = ["sample_id", "relative_file", "size_bytes", "sha256", "width", "height", "split"]
        rows = []
        for index in range(40):
            content = f"image-{index}".encode()
            relative = Path("Cars" if index < 38 else "Plates") / f"sample-{index}.jpg"
            path = image_root / relative
            path.parent.mkdir(exist_ok=True)
            path.write_bytes(content)
            rows.append({
                "sample_id": f"PKPLATE-{index + 1:04d}", "relative_file": relative.as_posix(),
                "size_bytes": len(content), "sha256": digest(content), "width": 100 + index,
                "height": 80 + index, "split": "sealed_holdout" if index < 22 else "development",
            })
        with (images_dir / "image-ground-truth-template.csv").open("w", encoding="utf-8-sig", newline="") as target:
            writer = csv.DictWriter(target, fieldnames=fields)
            writer.writeheader()
            writer.writerows(rows)
        (images_dir / "image-ground-truth-protocol.json").write_text("{}", encoding="utf-8")
        video = root / "sample.mp4"
        video_bytes = b"fake-private-video"
        video.write_bytes(video_bytes)
        (video_dir / "review-audio.m4a").write_bytes(b"fake-private-audio")
        (video_dir / "video-ground-truth-protocol.json").write_text(json.dumps({
            "source": {"size_bytes": len(video_bytes), "sha256": digest(video_bytes)},
        }), encoding="utf-8")
        return ReviewStore(repo_root=root, image_root=image_root, video=video, private_root=private_root)

    def test_selects_only_ten_development_and_existing_twenty_two_holdout(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            store = self.make_store(Path(temporary))
            public = store.public_state()
            self.assertEqual(10, len(public["images"]["development"]))
            self.assertEqual(22, len(public["images"]["sealed_holdout"]))
            self.assertEqual("GOLD_LABEL_ONLY", public["mode"])
            self.assertFalse(public["locked"])

    def test_seen_sample_is_recorded_excluded_and_deterministically_replaced(self) -> None:
        with tempfile.TemporaryDirectory() as first, tempfile.TemporaryDirectory() as second:
            store_a = self.make_store(Path(first))
            store_b = self.make_store(Path(second))
            sample_a = store_a.state["active"]["sealed_holdout"][0]
            sample_b = store_b.state["active"]["sealed_holdout"][0]
            self.assertEqual(sample_a, sample_b)
            store_a.action({"action": "mark_independence", "sample_id": sample_a, "model_output_seen": True})
            store_b.action({"action": "mark_independence", "sample_id": sample_b, "model_output_seen": True})
            excluded = store_a.state["excluded_exposed"][0]
            self.assertEqual("candidate_model_output_previously_seen", excluded["reason"])
            self.assertEqual(excluded["replacement_sample_id"], store_b.state["excluded_exposed"][0]["replacement_sample_id"])
            self.assertNotIn(sample_a, store_a.state["active"]["sealed_holdout"])

    def test_pending_silence_audit_migrates_to_six_useful_windows(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            store = self.make_store(root)
            store.state["vad_receipt"] = {
                "method": "browser_audio_energy_v1", "fallback": "digital_silence_audit",
                "audio_duration_seconds": 60.010,
            }
            store.state["speech_segments"] = [
                {"segment_id": f"SP-{index + 1:03d}", "start_seconds": index * 10,
                 "end_seconds": min(60.010, (index + 1) * 10), "review_status": "pending"}
                for index in range(7)
            ]
            store._save()
            reloaded = ReviewStore(
                repo_root=root, image_root=store.image_root, video=store.video,
                private_root=store.private_root,
            )
            self.assertEqual(6, len(reloaded.state["speech_segments"]))
            self.assertEqual(60.010, reloaded.state["speech_segments"][-1]["end_seconds"])

    def test_complete_human_workflow_locks_and_validates_without_model_fields(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            store = self.make_store(Path(temporary))
            store.action({"action": "set_reviewer", "reviewer": "Reviewer One"})
            for sample_id in store.state["active"]["development"]:
                store.action({"action": "mark_independence", "sample_id": sample_id, "model_output_seen": False})
                store.action({
                    "action": "save_image", "sample_id": sample_id, "plate_presence": "no",
                    "plate_count": 0, "plate_text": "", "readability": "not_applicable",
                    "difficulty": "", "review_notes": "",
                })
            store.action({"action": "freeze_development"})
            for sample_id in store.state["active"]["sealed_holdout"]:
                store.action({"action": "mark_independence", "sample_id": sample_id, "model_output_seen": False})
                store.action({
                    "action": "save_image", "sample_id": sample_id, "plate_presence": "no",
                    "plate_count": 0, "plate_text": "", "readability": "not_applicable",
                    "difficulty": "hard", "review_notes": "",
                })
            store.action({
                "action": "save_video_event", "start_seconds": 0, "end_seconds": 60.010,
                "event_type": "negative", "plate_count": 0, "plate_text": "",
                "readability": "not_applicable", "review_notes": "full negative",
            })
            store.action({"action": "set_video_review", "watched_full_source": True, "review_notes": "watched once"})
            store.action({
                "action": "set_speech_segments", "segments": [{"start_seconds": 1, "end_seconds": 2}],
                "receipt": {"method": "browser_audio_energy_v1"},
            })
            store.action({
                "action": "save_speech", "segment_id": "SP-001", "speech_present": "no",
                "transcript_raw": "", "transcript_language": "not_applicable",
                "unintelligible": False, "review_notes": "",
            })
            store.action({"action": "finish"})
            validation = json.loads((store.output_dir / "ground-truth-validation.json").read_text(encoding="utf-8"))
            self.assertEqual("PASS", validation["GroundTruthValidation"])
            self.assertTrue(store.state["locked"])
            self.assertFalse(validation["model_output_used_to_create_truth"])
            with self.assertRaisesRegex(ValueError, "locked"):
                store.action({"action": "set_reviewer", "reviewer": "Someone Else"})


if __name__ == "__main__":
    unittest.main()
