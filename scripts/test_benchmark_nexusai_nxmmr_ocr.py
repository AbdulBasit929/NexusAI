#!/usr/bin/env python3

import importlib.util
import unittest
from pathlib import Path


PATH = Path(__file__).with_name("benchmark_nexusai_nxmmr_ocr.py")
SPEC = importlib.util.spec_from_file_location("nxmmr_ocr", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader
SPEC.loader.exec_module(MODULE)


class OCRScoringTests(unittest.TestCase):
    def test_exact_identifier_normalization(self) -> None:
        result = MODULE.score("ICT-001", "ict 001")
        self.assertFalse(result["line_exact"])
        self.assertTrue(result["identifier_exact"])

    def test_edit_distance_is_not_self_oracle(self) -> None:
        result = MODULE.score("اسلام آباد", "اسلام")
        self.assertGreater(result["character_errors"], 0)
        self.assertGreater(result["word_errors"], 0)


if __name__ == "__main__":
    unittest.main()
