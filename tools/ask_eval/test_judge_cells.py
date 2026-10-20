"""Tests for the cell-set judge kind and the zero rule of the co-location questions (plain unittest):  python -X utf8 tools/ask_eval/test_judge_cells.py

Proves: a returned set of cell keys is right only when it is exactly the expected set (a missing cell, an extra cell of the case, or a value that is no cell key do not pass);
nothing expected is right only with no rows and the plain "no row" statement, and never with "no record of this case"; a count of zero is right as a row holding 0 or as the plain
"no row" statement when the question says an empty result is zero, and a wrong count, a "no record" or a refusal is not; a decline stays a decline.
"""
import os
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import judge  # noqa: E402

UNIVERSE = ["410-01-100-1", "410-01-100-2", "410-01-100-3", "410-01-100-4"]


def cells(values):
    return {"kind": "cells", "x": "923000000001", "y": "923000000002", "values": values, "universe": UNIVERSE}


def result(rows, answer="", state="answered"):
    return {"state": state, "rows": rows, "answer": answer}


class CellSetTests(unittest.TestCase):
    def test_the_exact_set_is_correct_in_any_column_and_order(self):
        q = {"expect": cells(["410-01-100-1", "410-01-100-3"])}
        self.assertEqual(judge.judge(q, result([{"cell_key": "410-01-100-3"}, {"cell_key": "410-01-100-1"}])), "correct")
        self.assertEqual(judge.judge(q, result([{"c": "410-01-100-1", "hour": "2026-01-01 10:00"}, {"c": "410-01-100-3", "hour": "2026-01-01 11:00"}, {"c": "410-01-100-1", "hour": "2026-01-02 09:00"}])), "correct")

    def test_a_missing_cell_an_extra_cell_or_no_cell_key_is_wrong(self):
        q = {"expect": cells(["410-01-100-1", "410-01-100-3"])}
        self.assertEqual(judge.judge(q, result([{"cell_key": "410-01-100-1"}])), "wrong")
        self.assertEqual(judge.judge(q, result([{"cell_key": "410-01-100-1"}, {"cell_key": "410-01-100-3"}, {"cell_key": "410-01-100-2"}])), "wrong")
        self.assertEqual(judge.judge(q, result([{"site_name": "Site 01"}])), "wrong")
        self.assertEqual(judge.judge(q, result([{"cell_key": "999-99-999-9"}])), "wrong")

    def test_nothing_expected_needs_no_rows_and_the_plain_statement(self):
        q = {"expect": cells([])}
        self.assertEqual(judge.judge(q, result([], "No row matches what you asked.")), "correct")
        self.assertEqual(judge.judge(q, result([], "No record of this case holds 923000000001 in any number, device, SIM, address or cell column.")), "wrong")
        self.assertEqual(judge.judge(q, result([{"cell_key": "410-01-100-1"}], "1 row.")), "wrong")

    def test_a_decline_or_an_abstention_is_not_wrong_and_not_right(self):
        q = {"expect": cells(["410-01-100-1"])}
        self.assertEqual(judge.judge(q, result([], "", "declined")), "declined")
        self.assertEqual(judge.judge(q, result([], "", "abstained")), "abstained")

    def test_a_count_of_zero_may_be_an_empty_result_only_when_the_question_says_so(self):
        zero = {"expect": {"kind": "scalar", "value": 0, "empty_is_zero": True}}
        self.assertEqual(judge.judge(zero, result([{"n": 0}], "0")), "correct")
        self.assertEqual(judge.judge(zero, result([], "No row matches what you asked.")), "correct")
        self.assertEqual(judge.judge(zero, result([], "No record of this case holds 923000000001 in any number, device, SIM, address or cell column.")), "wrong")
        self.assertEqual(judge.judge(zero, result([{"n": 3}], "3")), "wrong")
        plain = {"expect": {"kind": "scalar", "value": 0}}
        self.assertEqual(judge.judge(plain, result([], "No row matches what you asked.")), "wrong")  # the older kinds are unchanged
        self.assertEqual(judge.judge({"expect": {"kind": "scalar", "value": 4, "empty_is_zero": True}}, result([{"n": 4}], "4")), "correct")


if __name__ == "__main__":
    unittest.main()
