import io
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from scripts.download_nexusai_r8_artifact import download


class FakeResponse(io.BytesIO):
    def __init__(self, payload: bytes, status: int, headers: dict[str, str]):
        super().__init__(payload)
        self.status = status
        self.headers = headers

    def getcode(self):
        return self.status

    def __enter__(self):
        return self

    def __exit__(self, *_args):
        self.close()


class ResumableDownloaderTests(unittest.TestCase):
    def test_resumes_only_from_matching_content_range(self):
        with tempfile.TemporaryDirectory() as temp:
            destination = Path(temp, "partial")
            destination.write_bytes(b"abc")
            response = FakeResponse(b"def", 206, {"Content-Range": "bytes 3-5/6"})
            with mock.patch("urllib.request.urlopen", return_value=response):
                result = download("https://example.invalid/file", destination, 6, attempts=1)
            self.assertEqual(destination.read_bytes(), b"abcdef")
            self.assertEqual(result["status"], "pass")

    def test_rejects_server_that_ignores_resume(self):
        with tempfile.TemporaryDirectory() as temp:
            destination = Path(temp, "partial")
            destination.write_bytes(b"abc")
            response = FakeResponse(b"abcdef", 200, {})
            with mock.patch("urllib.request.urlopen", return_value=response):
                with self.assertRaisesRegex(RuntimeError, "ignored resume range"):
                    download("https://example.invalid/file", destination, 6, attempts=1)
            self.assertEqual(destination.read_bytes(), b"abc")


if __name__ == "__main__":
    unittest.main()
