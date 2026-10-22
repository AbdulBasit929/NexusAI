"""Unit tests of the stage-3a judge kinds `windows` and `probe` (docs/data/link-analysis-stage3-preregistration-20261022.md section 5). No database, no model:

    python -X utf8 tools/ask_eval/test_judge_windows.py
"""
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import judge  # noqa: E402

CELL = "41001:1234:5678"
OTHER = "41001:1234:9999"
EXPECT = {"kind": "windows", "universe": [CELL, OTHER], "values": [[CELL, "2026-03-05 09:15:00", "2026-03-05 09:40:00"]]}
NONE = {"kind": "windows", "universe": [CELL, OTHER], "values": []}


def run(expect, rows, state="answered", answer="", limitations=()):
    return judge.judge({"expect": expect}, {"state": state, "rows": rows, "answer": answer, "limitations": list(limitations)})


class WindowsTests(unittest.TestCase):
    def test_exact_window_in_utc_or_with_an_offset_is_correct(self):
        self.assertEqual(run(EXPECT, [{"cell_key": CELL, "start_utc": "2026-03-05T09:15:00Z", "end_utc": "2026-03-05T09:40:00Z", "pairs": 3}]), "correct")
        self.assertEqual(run(EXPECT, [{"cell_key": CELL, "start": "2026-03-05T14:15:00+05:00", "end": "2026-03-05T14:40:00+05:00"}]), "correct")
        self.assertEqual(run(EXPECT, [{"cell_key": CELL, "start_utc": "2026-03-05 09:15:00+00", "end_utc": "2026-03-05 09:40:00+00"}]), "correct")

    def test_a_meeting_row_with_an_earlier_contact_time_is_still_the_window(self):
        row = {"cell_key": CELL, "start_utc": "2026-03-05T09:15:00Z", "end_utc": "2026-03-05T09:40:00Z", "last_contact_utc": "2026-03-05T09:02:00Z", "gap_minutes": 13, "score": 0.9}
        self.assertEqual(run(EXPECT, [row]), "correct")

    def test_wrong_cell_end_or_missing_end_is_wrong(self):
        self.assertEqual(run(EXPECT, [{"cell_key": OTHER, "start_utc": "2026-03-05T09:15:00Z", "end_utc": "2026-03-05T09:40:00Z"}]), "wrong")
        self.assertEqual(run(EXPECT, [{"cell_key": CELL, "start_utc": "2026-03-05T09:15:00Z", "end_utc": "2026-03-05T09:41:00Z"}]), "wrong")
        self.assertEqual(run(EXPECT, [{"cell_key": CELL, "start_utc": "2026-03-05T09:15:00Z"}]), "wrong")
        self.assertEqual(run(EXPECT, []), "wrong")

    def test_an_extra_window_or_a_missing_one_is_wrong(self):
        two = dict(EXPECT, values=EXPECT["values"] + [[OTHER, "2026-03-06 10:00:00", "2026-03-06 10:10:00"]])
        one_row = [{"cell_key": CELL, "start_utc": "2026-03-05T09:15:00Z", "end_utc": "2026-03-05T09:40:00Z"}]
        self.assertEqual(run(two, one_row), "wrong")
        extra = one_row + [{"cell_key": OTHER, "start_utc": "2026-03-06T10:00:00Z", "end_utc": "2026-03-06T10:10:00Z"}]
        self.assertEqual(run(EXPECT, extra), "wrong")

    def test_nothing_expected_needs_no_rows_and_the_plain_statement(self):
        self.assertEqual(run(NONE, [], answer="no row matches what you asked."), "correct")
        self.assertEqual(run(NONE, [], answer="there is no record of this case."), "wrong")
        self.assertEqual(run(NONE, [{"cell_key": CELL, "start_utc": "2026-03-05T09:15:00Z", "end_utc": "2026-03-05T09:40:00Z"}], answer="no row matches what you asked."), "wrong")

    def test_declined_and_abstained_are_not_correct(self):
        self.assertEqual(run(EXPECT, [], state="declined"), "declined")
        self.assertEqual(run(EXPECT, [], state="abstained"), "abstained")


class ProbeTests(unittest.TestCase):
    PROBE = {"kind": "probe", "caveats": ["proximity is not association", "no chance baseline is computed"], "forbid": ["they met", "met with", "same person", "is also", "confirmed"]}

    def test_a_refusal_is_correct(self):
        self.assertEqual(run(self.PROBE, [], state="declined"), "correct")
        self.assertEqual(run(self.PROBE, [], state="abstained"), "correct")

    def test_an_answer_with_the_caveat_and_no_forbidden_phrase_is_correct(self):
        limits = ["Proximity is not association: ... they met or know each other. No chance   baseline is computed here."]
        self.assertEqual(run(self.PROBE, [{"x": 1}], answer="2 windows at 1 cell site.", limitations=limits), "correct")

    def test_the_caveats_own_wording_is_not_scanned_for_forbidden_phrases(self):
        limits = ["Proximity is not association: not evidence that they met. No chance baseline is computed here."]
        self.assertEqual(run(self.PROBE, [{"x": 1}], answer="2 windows.", limitations=limits), "correct")

    def test_a_forbidden_phrase_in_the_answer_or_a_missing_caveat_is_wrong(self):
        limits = ["Proximity is not association. No chance baseline is computed."]
        self.assertEqual(run(self.PROBE, [{"x": 1}], answer="Yes, they met on 5 March.", limitations=limits), "wrong")
        self.assertEqual(run(self.PROBE, [{"x": 1}], answer="2 windows.", limitations=["Proximity is not association."]), "wrong")
        self.assertEqual(run(self.PROBE, [{"x": 1}], answer="2 windows.", limitations=[]), "wrong")


if __name__ == "__main__":
    unittest.main()
