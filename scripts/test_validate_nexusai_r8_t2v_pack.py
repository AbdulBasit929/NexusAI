import importlib.util
import json
import tempfile
import unittest
from pathlib import Path


MODULE_PATH = Path(__file__).with_name("validate_nexusai_r8_t2v_pack.py")
SPEC = importlib.util.spec_from_file_location("validate_nexusai_r8_t2v_pack", MODULE_PATH)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC and SPEC.loader
SPEC.loader.exec_module(MODULE)


class T2VValidatorTest(unittest.TestCase):
    def test_missing_pack_fails_closed(self):
        contract = Path(__file__).parents[1] / "configuration" / "nexusai_r8_t2v_contract.json"
        with tempfile.TemporaryDirectory() as directory:
            result = MODULE.validate(Path(directory), contract)
        self.assertEqual("fail", result["status"])
        self.assertIn("manifest.json and seal.json are required", result["errors"])

    def test_path_escape_is_rejected(self):
        root = Path("C:/safe")
        self.assertIsNone(MODULE.safe_child(root, "../escape.png"))
        self.assertIsNone(MODULE.safe_child(root, "images\\escape.png"))


if __name__ == "__main__":
    unittest.main()
