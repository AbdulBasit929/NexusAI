"""Unit tests of the `set` judge's reading of identifiers (stage 3 milestone run, 2026-10-23): the IMSI column of a row of handsets is not a handset. No database, no model:

    python -X utf8 tools/ask_eval/test_judge_set.py
"""
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import judge  # noqa: E402


def run(values, rows):
    return judge.judge({"expect": {"kind": "set", "values": values}}, {"state": "answered", "rows": rows, "answer": ""})


class SetTests(unittest.TestCase):
    def test_a_list_of_handsets_with_the_imsi_column_is_right(self):
        rows = [{"imei": "00000000001007", "imsi": "001010000001007", "events": 506}, {"imei": "00000000020001", "imsi": "001010000001007", "events": 215}]
        self.assertEqual(run(["00000000001007", "00000000020001"], rows), "correct")

    def test_a_missing_or_extra_handset_is_still_wrong(self):
        rows = [{"imei": "00000000001007", "imsi": "001010000001007"}]
        self.assertEqual(run(["00000000001007", "00000000020001"], rows), "wrong")
        rows = [{"imei": "00000000001007"}, {"imei": "00000000020001"}, {"imei": "00000000030001"}]
        self.assertEqual(run(["00000000001007", "00000000020001"], rows), "wrong")

    def test_a_15_digit_imei_is_its_14_digits(self):
        self.assertEqual(run(["00000000001007"], [{"imei": "000000000010071"}]), "correct")

    def test_other_digit_columns_still_count(self):
        # a number in a column that is not the IMSI is an identifier the answer gave
        self.assertEqual(run(["00000000001007"], [{"imei": "00000000001007", "src": "923000001007"}]), "wrong")


if __name__ == "__main__":
    unittest.main()
