"""Tests for the case set and its register (plain unittest):  python -X utf8 tools/case_generator/test_case_set.py

Proves: the set is 33 cases (judgement 109 to 112 were added on 2026-10-22, 113 to 120 on 2026-10-26 and 121 on 2026-10-29, each batch before the measurement it serves) with seeds in the range of their kind and no seed twice; a malformed set is refused; the register is the same
every time it is made and every case in it passes the audit; `verify` notices a changed case, a changed generator and, for sealed cases,
only when asked; and `build` does not write a sealed case without the gate flag.
"""
import copy
import io
import os
import sys
import tempfile
import unittest
from contextlib import redirect_stdout

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import case_set  # noqa: E402
import world as w  # noqa: E402

TINY = dict(people=40, subjects=4, associates=8, period_days=14, p2_pairs=3)
TINY_SET = [("development", 1, dict(TINY)), ("judgement", 101, dict(TINY, period_days=12)), ("sealed", 901, dict(TINY, p2_pairs=2))]


class SetTests(unittest.TestCase):
    def test_twenty_cases_in_three_kinds_with_seeds_in_range(self):
        self.assertEqual(case_set.validate(), [])
        self.assertEqual(len(case_set.CASES), 33)
        counts = {kind: sum(1 for k, _, _ in case_set.CASES if k == kind) for kind in case_set.KINDS}
        self.assertEqual(counts, {"development": 8, "judgement": 21, "sealed": 4})
        for kind, seed, overrides in case_set.CASES:
            self.assertIn(seed, case_set.KINDS[kind])
            case_set.params_for(seed, overrides)      # every override is a parameter

    def test_a_malformed_set_is_refused(self):
        self.assertTrue(any("twice" in p for p in case_set.validate([("development", 1, {}), ("development", 1, {})])))
        self.assertTrue(any("outside the range" in p for p in case_set.validate([("sealed", 5, {})])))
        self.assertTrue(any("outside the range" in p for p in case_set.validate([("judgement", 99, {})])))
        self.assertTrue(any("unknown parameters" in p for p in case_set.validate([("development", 2, {"nonsense": 1})])))
        self.assertTrue(any("outside the range" in p for p in case_set.validate([("other", 2, {})])))

    def test_the_sets_differ_in_scale(self):
        sizes = {(o.get("subjects", 12), o.get("period_days", 90), o.get("people", 120)) for _, _, o in case_set.CASES}
        self.assertGreaterEqual(len(sizes), 8)


class RegisterTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.first, cls.failures = case_set.build_register(TINY_SET)

    def test_the_register_is_the_same_every_time_and_every_case_passes_the_audit(self):
        self.assertEqual(self.failures, [])
        self.assertEqual([e["audit_problems"] for e in self.first["cases"]], [0, 0, 0])
        again, _ = case_set.build_register(TINY_SET)
        self.assertEqual(again, self.first)
        self.assertEqual(len({e["manifest_sha256"] for e in self.first["cases"]}), 3, "different cases, different hashes")
        self.assertEqual(self.first["generator_version"], w.GENERATOR_VERSION)

    def test_verify_notices_a_changed_case_and_a_changed_generator(self):
        self.assertEqual(case_set.verify_register(self.first, kinds=("development", "judgement", "sealed")), [])
        changed = copy.deepcopy(self.first)
        changed["cases"][0]["overrides"]["p2_pairs"] = 2
        self.assertTrue(any("development-1" in d for d in case_set.verify_register(changed)))
        older = copy.deepcopy(self.first)
        older["generator_version"] = "0.0"
        self.assertTrue(any("0.0" in d for d in case_set.verify_register(older)))

    def test_sealed_cases_are_checked_only_when_asked(self):
        broken = copy.deepcopy(self.first)
        broken["cases"][2]["key_sha256"] = "0" * 64
        self.assertEqual(case_set.verify_register(broken), [])
        self.assertTrue(any("sealed-901" in d for d in case_set.verify_register(broken, kinds=("sealed",))))


class CommandLineTests(unittest.TestCase):
    def test_build_refuses_sealed_cases_without_the_gate_flag(self):
        with tempfile.TemporaryDirectory() as d:
            out = io.StringIO()
            with redirect_stdout(out):
                status = case_set.main(["build", "--out", d, "--kind", "sealed"])
            self.assertEqual(status, 3)
            self.assertEqual(os.listdir(d), [])
            with redirect_stdout(io.StringIO()):
                status = case_set.main(["build", "--out", d, "--kind", "all"])
            self.assertEqual(status, 3, "all includes the sealed cases")
            self.assertEqual(os.listdir(d), [])

    def test_plan_lists_the_set(self):
        out = io.StringIO()
        with redirect_stdout(out):
            self.assertEqual(case_set.main(["plan"]), 0)
        self.assertIn("33 cases: 8 development, 21 judgement, 4 sealed", out.getvalue())


if __name__ == "__main__":
    unittest.main()
