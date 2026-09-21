# SPDX-License-Identifier: MIT
"""Non-retained, dependency-free ASR provenance and language contract checks."""

import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from ingestion.forensic_records.media_pipeline import LocalAIASRProcessor, MediaProcessingError


class ASRTimingIntegrityTests(unittest.TestCase):
    def transcribe(self, result, language=None):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "speech.wav"
            source.write_bytes(b"test-only-audio")
            with patch("ingestion.forensic_records.media_pipeline.urllib.request.urlopen") as request:
                request.return_value.__enter__.return_value.read.return_value = json.dumps(result).encode()
                observations = LocalAIASRProcessor("http://127.0.0.1:8080/v1/audio/transcriptions", "small").transcribe(
                    source, evidence_id="evidence-1", version_id="version-1",
                    source_file="speech.wav", language=language,
                )
                self.request_body = request.call_args.args[0].data
                return observations

    def test_text_only_response_never_inherits_clip_duration(self):
        for segments in [None, [], "invalid"]:
            with self.subTest(segments=segments):
                observation, = self.transcribe({"text": "spoken text", "duration": 60, "segments": segments})
                self.assertEqual(observation.payload["text"], "spoken text")
                self.assertEqual(observation.citation_locator, {"source_file": "speech.wav", "start_seconds": None, "end_seconds": None})
                self.assertEqual(observation.payload["timing_status"], "unavailable_or_partial")
                self.assertEqual(len(observation.warnings), 2)

    def test_empty_speech_is_a_real_zero_result(self):
        self.assertEqual(self.transcribe({"text": " ", "segments": []}), ())

    def test_malformed_response_is_not_a_zero_result(self):
        with self.assertRaisesRegex(MediaProcessingError, "must be an object"):
            self.transcribe(["invalid"])

    def test_reported_times_and_explicit_languages_are_preserved(self):
        for language in ["en", "ur"]:
            with self.subTest(language=language):
                observation, = self.transcribe({"language": language, "segments": [{"start": 1.25, "end": 2.5, "text": "bounded"}]}, language)
                self.assertEqual(observation.citation_locator["start_seconds"], 1.25)
                self.assertEqual(observation.citation_locator["end_seconds"], 2.5)
                self.assertEqual(observation.payload["timing_status"], "reported")
                self.assertIsNone(observation.payload["detected_language"])
                self.assertIn(f'\r\n\r\n{language}\r\n'.encode(), self.request_body)

    def test_auto_language_does_not_claim_an_explicit_selection(self):
        observation, = self.transcribe({"language": "en", "segments": [{"start": 0, "end": 1, "text": "spoken"}]})
        self.assertEqual(observation.payload["detected_language"], "en")
        self.assertNotIn(b'name="language"', self.request_body)

    def test_invalid_times_are_unknown_and_raw_values_survive(self):
        for value in [None, True, False, -1, "NaN", "Infinity", "invalid"]:
            with self.subTest(value=value):
                observation, = self.transcribe({"segments": [{"start": value, "end": 2, "text": "spoken"}]})
                self.assertIsNone(observation.citation_locator["start_seconds"])
                self.assertEqual(observation.payload["raw_start"], value)
                self.assertEqual(observation.payload["timing_status"], "unavailable_or_partial")

    def test_reversed_range_is_not_a_valid_citation(self):
        observation, = self.transcribe({"segments": [{"start": 4, "end": 2, "text": "spoken"}]})
        self.assertIsNone(observation.payload["start_seconds"])
        self.assertIsNone(observation.payload["end_seconds"])
        self.assertEqual((observation.payload["raw_start"], observation.payload["raw_end"]), (4, 2))

    def test_legacy_nanoseconds_remain_compatible(self):
        observation, = self.transcribe({"segments": [{"start": 1250000000, "end": 2500000000, "text": "spoken"}]})
        self.assertEqual((observation.payload["start_seconds"], observation.payload["end_seconds"]), (1.25, 2.5))

    def test_untimed_urdu_derivative_preserves_parent_and_identifiers(self):
        raw = "اس آڈیو میں 03001234567 اور MN1367 کا ذکر 10:35 پر ہوا"
        transcript, roman = self.transcribe({"language": "ur", "text": raw, "duration": 6.24}, "ur")
        self.assertEqual(roman.payload["raw_urdu_text"], raw)
        self.assertEqual(roman.payload["parent_observation_id"], transcript.observation_id)
        self.assertEqual(roman.citation_locator, transcript.citation_locator)
        self.assertIsNone(roman.payload["start_seconds"])
        self.assertTrue(roman.payload["identifier_preservation_pass"])
        for identifier in ["03001234567", "MN1367", "10:35"]:
            self.assertIn(identifier, roman.payload["roman_urdu_text"])


if __name__ == "__main__":
    unittest.main()
