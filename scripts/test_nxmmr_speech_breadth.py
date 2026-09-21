# SPDX-License-Identifier: MIT
import io
import tempfile
import unittest
import wave
from pathlib import Path
from unittest.mock import MagicMock, patch

from scripts.nxmmr_speech_breadth import REVISION, admitted_license, aggregate, canonical, fetch, save_new, score, sha, verify, wav_info


class SpeechBreadthTests(unittest.TestCase):
    def test_license_receipt_accepts_only_the_exact_official_license(self):
        self.assertTrue(admitted_license("cc-by-4.0"))
        self.assertTrue(admitted_license(["cc-by-4.0"]))
        self.assertFalse(admitted_license(["cc-by-4.0", "unknown"]))
        self.assertFalse(admitted_license(None))

    def test_scorer_uses_independent_edit_counts(self):
        result = score("one two three four", "one too three")
        self.assertEqual(result["word_edits"], 2)
        self.assertEqual(result["reference_words"], 4)
        self.assertFalse(result["normalized_exact"])

    def test_normalization_and_urdu_identifier_preservation(self):
        self.assertTrue(score("HELLO, world!", "hello world")["normalized_exact"])
        result = score("یہ 03001234567 ہے", "یہ 03001234567 ہے")
        self.assertEqual(result["word_edits"], 0)
        self.assertEqual(result["reference_number_tokens"], ["03001234567"])

    def test_micro_aggregation_includes_failed_clip_deletions(self):
        rows = [
            {"duration_seconds": 2, "wall_seconds": 1, "error": None, "hypothesis": "a b", "timing_complete": True, "score": score("a b", "a b")},
            {"duration_seconds": 3, "wall_seconds": 2, "error": "failure", "hypothesis": "", "timing_complete": False, "score": score("c", "")},
        ]
        result = aggregate(rows)
        self.assertEqual(result["wer"], 1 / 3)
        self.assertEqual(result["cer"], 1 / 3)
        self.assertEqual(result["failures"], 1)
        self.assertEqual(result["normalized_exact_sentence_rate"], 0.5)

    def test_manifest_is_immutable_and_digest_detects_change(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "manifest.json"
            save_new(path, {"rows": [0, 1]})
            save_new(path, {"rows": [0, 1]})
            with self.assertRaises(ValueError):
                save_new(path, {"rows": [1, 2]})
            self.assertNotEqual(sha(canonical({"rows": [0, 1]})), sha(canonical({"rows": [1, 2]})))

    def test_wav_metadata_matches_real_frames(self):
        buffer = io.BytesIO()
        with wave.open(buffer, "wb") as audio:
            audio.setnchannels(1)
            audio.setsampwidth(2)
            audio.setframerate(16000)
            audio.writeframes(b"\0\0" * 16000)
        self.assertEqual(wav_info(buffer.getvalue())["duration_seconds"], 1)
        with self.assertRaises(ValueError):
            wav_info(buffer.getvalue()[:-10])

    def test_unapproved_sources_rejected_before_network(self):
        for url in ["http://datasets-server.huggingface.co/assets/x", "https://example.com/audio.wav", "https://datasets-server.huggingface.co/assets/wrong-revision"]:
            with self.subTest(url=url), self.assertRaises(ValueError):
                fetch(url, 100, audio=True)

    def test_pinned_official_asset_path_and_response_byte_limit(self):
        url = f"https://datasets-server.huggingface.co/cached-assets/google/fleurs/--/{REVISION}/--/en_us/validation/0/audio/audio.wav"
        response = MagicMock()
        response.headers = {}
        response.read.return_value = b"sample"
        opener = MagicMock()
        opener.open.return_value.__enter__.return_value = response
        with patch("scripts.nxmmr_speech_breadth.urllib.request.build_opener", return_value=opener):
            self.assertEqual(fetch(url, 6, audio=True), b"sample")
            with self.assertRaisesRegex(ValueError, "byte allowance"):
                fetch(url, 5, audio=True)

    def test_acquisition_rejects_changed_seal_without_assert_statements(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            selection = {"clips": []}
            save_new(root / "selection.json", selection)
            save_new(root / "acquisition.json", {"selection_sha256": "changed"})
            with patch("scripts.nxmmr_speech_breadth.load_selection", return_value=(root, selection)):
                with self.assertRaisesRegex(ValueError, "selection digest mismatch"):
                    verify()


if __name__ == "__main__":
    unittest.main()
