import json
import tempfile
import unittest
from pathlib import Path

from scripts.check_nexusai_r8_t2_capture_files import inspect_pack


class T2CaptureFilesTest(unittest.TestCase):
    def test_complete_unique_files_pass_and_duplicates_fail(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            images = root / "images"
            images.mkdir()
            plan = {
                "items": [
                    {"image_id": "a", "filename": "images/a.jpg", "partition": "development"},
                    {"image_id": "b", "filename": "images/b.jpg", "partition": "holdout"},
                ]
            }
            (root / "capture-plan.json").write_text(json.dumps(plan), encoding="utf-8")
            (images / "a.jpg").write_bytes(b"\xff\xd8\xfffirst")
            (images / "b.jpg").write_bytes(b"\xff\xd8\xffsecond")
            result = inspect_pack(root)
            self.assertTrue(result["capture_complete"])
            self.assertEqual(1, result["development_file_count"])
            self.assertEqual(1, result["holdout_file_count"])
            (images / "b.jpg").write_bytes((images / "a.jpg").read_bytes())
            result = inspect_pack(root)
            self.assertFalse(result["capture_complete"])
            self.assertEqual(1, len(result["duplicate_content_groups"]))


if __name__ == "__main__":
    unittest.main()
