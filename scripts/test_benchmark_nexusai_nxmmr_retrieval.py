#!/usr/bin/env python3

import importlib.util
import unittest
from pathlib import Path


PATH = Path(__file__).with_name("benchmark_nexusai_nxmmr_retrieval.py")
SPEC = importlib.util.spec_from_file_location("nxmmr_retrieval", PATH)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader
SPEC.loader.exec_module(MODULE)


class RetrievalMetricTests(unittest.TestCase):
    def test_exact_first_rank(self) -> None:
        rows = [{
            "expected_document_ids": ["a"], "ranked_document_ids": ["a", "b"],
            "abstained": False,
        }]
        result = MODULE.metrics(rows)
        self.assertEqual(1.0, result["recall_at_1"])
        self.assertEqual(1.0, result["mrr"])

    def test_no_answer_abstention(self) -> None:
        rows = [{"expected_document_ids": [], "ranked_document_ids": ["a"], "abstained": True}]
        self.assertEqual(1.0, MODULE.metrics(rows)["no_answer_abstention_accuracy"])


if __name__ == "__main__":
    unittest.main()
