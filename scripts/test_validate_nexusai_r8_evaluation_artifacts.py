import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from validate_nexusai_r8_evaluation_artifacts import validate


ROOT = Path(__file__).resolve().parents[1]
MANIFEST = ROOT / "configuration" / "nexusai_r8_evaluation_artifacts.json"


class R8AcquisitionManifestTests(unittest.TestCase):
    def test_repository_manifest_passes(self):
        result = validate(MANIFEST)
        self.assertEqual(result, {"status": "pass", "candidates": 5, "artifacts": 8})

    def test_production_authority_fails_closed(self):
        payload = json.loads(MANIFEST.read_text(encoding="utf-8"))
        payload["production_role_assignment"] = True
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "manifest.json"
            path.write_text(json.dumps(payload), encoding="utf-8")
            with self.assertRaises(AssertionError):
                validate(path)

    def test_unapproved_host_fails_closed(self):
        payload = json.loads(MANIFEST.read_text(encoding="utf-8"))
        payload["candidates"][0]["artifacts"][0]["url"] = "https://example.invalid/model.bin"
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "manifest.json"
            path.write_text(json.dumps(payload), encoding="utf-8")
            with self.assertRaises(AssertionError):
                validate(path)


if __name__ == "__main__":
    unittest.main()
