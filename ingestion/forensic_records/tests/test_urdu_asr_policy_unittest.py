import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from ingestion.forensic_records.media_pipeline import LocalAIASRProcessor, MediaProcessingError


class _Response:
    def __init__(self, text: str):
        self._text = text

    def __enter__(self):
        return self

    def __exit__(self, *_args):
        return False

    def read(self):
        return json.dumps({
            "language": "ur",
            "segments": [{"start": 0, "end": 1, "text": self._text}],
        }).encode()


class TrustedUrduASRPolicyTests(unittest.TestCase):
    def transcribe(self, text: str):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "speech.wav"
            source.write_bytes(b"bounded-audio")
            processor = LocalAIASRProcessor(
                "http://127.0.0.1:8080/v1/audio/transcriptions", "small"
            )
            with patch(
                "ingestion.forensic_records.media_pipeline.urllib.request.urlopen",
                return_value=_Response(text),
            ):
                return processor.transcribe(
                    source,
                    evidence_id="evidence-1",
                    version_id="version-1",
                    source_file="speech.wav",
                    language="ur",
                )

    def test_accepts_urdu_script_and_creates_roman_derivative(self):
        observations = self.transcribe("یہ ایک اردو جملہ ہے")
        self.assertEqual(len(observations), 2)
        self.assertEqual(observations[0].payload["requested_language"], "ur")
        self.assertEqual(observations[1].payload["source_language"], "ur")

    def test_rejects_devanagari_for_trusted_urdu(self):
        with self.assertRaisesRegex(MediaProcessingError, "Devanagari"):
            self.transcribe("यह देवनागरी है")

    def test_rejects_latin_only_for_trusted_urdu(self):
        with self.assertRaisesRegex(MediaProcessingError, "did not return Urdu script"):
            self.transcribe("latin-only output")


if __name__ == "__main__":
    unittest.main()
