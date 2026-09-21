import io
import subprocess
import sys
import tarfile
import tempfile
import unittest
import zipfile
from pathlib import Path


SCRIPT = Path(__file__).with_name("validate_nexusai_r8_t3_archive.py")


def run_validator(path: Path) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(SCRIPT), "--archive", str(path), "--expected-bytes", str(path.stat().st_size)],
        capture_output=True,
        text=True,
        check=False,
    )


class ArchiveValidatorTests(unittest.TestCase):
    def test_accepts_benign_tar(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp, "ok.tar")
            with tarfile.open(path, "w") as archive:
                payload = b"image"
                info = tarfile.TarInfo("CCPD/base/example.jpg")
                info.size = len(payload)
                archive.addfile(info, io.BytesIO(payload))
            result = run_validator(path)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("R8T3ArchiveValidation=PASS", result.stdout)

    def test_rejects_traversal_zip(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp, "bad.zip")
            with zipfile.ZipFile(path, "w") as archive:
                archive.writestr("../escape.jpg", b"image")
            result = run_validator(path)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("unsafe_path_component", result.stderr)

    def test_rejects_executable_zip(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp, "bad.zip")
            with zipfile.ZipFile(path, "w") as archive:
                archive.writestr("CCPD/run.exe", b"MZ")
            result = run_validator(path)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("unexpected_executable_member", result.stderr)


if __name__ == "__main__":
    unittest.main()
