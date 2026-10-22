"""Unit tests of the stage-3b/3c judge kind `tuples` (docs/data/link-analysis-stage3-preregistration-20261022.md section 5). No database, no model:

    python -X utf8 tools/ask_eval/test_judge_tuples.py
"""
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import judge  # noqa: E402


def run(values, rows, state="answered", answer="", tol=None):
    expect = {"kind": "tuples", "values": values}
    if tol is not None:
        expect["tol"] = tol
    return judge.judge({"expect": expect}, {"state": state, "rows": rows, "answer": answer})


class TuplesTests(unittest.TestCase):
    def test_a_row_holding_all_the_values_is_correct_whatever_else_it_holds(self):
        want = [["923000001003", 0.5123]]
        self.assertEqual(run(want, [{"new_number": "923000001003", "score": 0.5123, "shared_device": True, "cell_jaccard": 0.1}], tol=0.001), "correct")
        self.assertEqual(run(want, [{"score": 0.5123, "other": "923000001003"}], tol=0.001), "correct")

    def test_a_wrong_number_or_a_score_outside_the_tolerance_is_wrong(self):
        want = [["923000001003", 0.5123]]
        self.assertEqual(run(want, [{"new_number": "923000001004", "score": 0.5123}], tol=0.001), "wrong")
        self.assertEqual(run(want, [{"new_number": "923000001003", "score": 0.52}], tol=0.001), "wrong")

    def test_none_missing_and_none_extra(self):
        want = [["a", 1], ["b", 2]]
        self.assertEqual(run(want, [{"x": "a", "n": 1}]), "wrong")
        self.assertEqual(run(want, [{"x": "a", "n": 1}, {"x": "b", "n": 2}, {"x": "c", "n": 3}]), "wrong")
        self.assertEqual(run(want, [{"x": "b", "n": 2}, {"x": "a", "n": 1}]), "correct")
        self.assertEqual(run(want, [{"x": "a", "n": 1}, {"x": "a", "n": 1}]), "wrong")

    def test_instants_dates_integers_floats_and_nulls(self):
        want = [["2026-03-05 03:36:08", "2026-03-02", 3, 12.34, None, "up"]]
        row = {"t": "2026-03-05 08:36:08 PKT", "d": "2026-03-02", "w": 3.0, "km": 12.341, "speed": None, "dir": "up"}
        self.assertEqual(run(want, [row]), "correct")
        self.assertEqual(run(want, [dict(row, t="2026-03-05T03:36:09Z")]), "wrong")
        self.assertEqual(run(want, [dict(row, d="2026-03-03")]), "wrong")
        self.assertEqual(run(want, [dict(row, w=4)]), "wrong")
        self.assertEqual(run(want, [dict(row, km=12.4)]), "wrong")
        self.assertEqual(run(want, [dict(row, speed=151.0)]), "wrong")

    def test_a_date_may_come_back_as_an_instant(self):
        self.assertEqual(run([["2026-03-02"]], [{"d": "2026-03-02T00:00:00+05:00"}]), "correct")

    def test_booleans_are_not_numbers(self):
        self.assertEqual(run([[1]], [{"b": True}]), "wrong")
        self.assertEqual(run([[True]], [{"b": True}]), "correct")

    def test_nothing_expected_needs_no_rows_and_the_plain_statement(self):
        self.assertEqual(run([], [], answer="no row matches what you asked."), "correct")
        self.assertEqual(run([], [], answer="there is no record of this case."), "wrong")
        self.assertEqual(run([], [{"x": 1}], answer="no row matches what you asked."), "wrong")

    def test_declined_and_abstained_are_not_correct(self):
        self.assertEqual(run([["a"]], [], state="declined"), "declined")
        self.assertEqual(run([["a"]], [], state="abstained"), "abstained")


if __name__ == "__main__":
    unittest.main()
