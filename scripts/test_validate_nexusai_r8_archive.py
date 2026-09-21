import hashlib
import tempfile
import unittest
from pathlib import Path
from zipfile import ZipFile

from scripts.validate_nexusai_r8_archive import validate_archive


class R8ArchiveValidationTests(unittest.TestCase):
    def test_single_expected_member_passes(self):
        content = b"governed-model-weight"
        with tempfile.TemporaryDirectory() as directory:
            archive = Path(directory) / "model.zip"
            with ZipFile(archive, "w") as zipped:
                zipped.writestr("model.pth", content)
            result = validate_archive(
                archive, "model.pth", len(content), "MD5", hashlib.md5(content).hexdigest()
            )
            self.assertEqual(result["status"], "pass")

    def test_path_traversal_fails(self):
        content = b"unsafe"
        with tempfile.TemporaryDirectory() as directory:
            archive = Path(directory) / "model.zip"
            with ZipFile(archive, "w") as zipped:
                zipped.writestr("../model.pth", content)
            with self.assertRaises(ValueError):
                validate_archive(
                    archive, "../model.pth", len(content), "MD5", hashlib.md5(content).hexdigest()
                )


if __name__ == "__main__":
    unittest.main()
