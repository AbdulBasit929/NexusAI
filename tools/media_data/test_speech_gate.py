"""Tests of tools/media_data/speech_gate.py: the status follows the registered gates mechanically.

    python -I -X utf8 tools/media_data/test_speech_gate.py
"""
import os
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)

import speech_gate as sg  # noqa: E402


def row(w_edits, w_len, c_edits=1, c_len=50, ref="كتاب سړي اوبه", hyp="كتاب سړي اوبه", script=1.0, loop=False):
    part = {"w_edits": w_edits, "w_len": w_len, "c_edits": c_edits, "c_len": c_len}
    return {"seconds": 10.0, "compute": 3.5, "script": script, "loop": loop, "empty": False, "reference": ref, "hypothesis": hyp, "N1": part, "N2": part}


def run(set_name, rows):
    return {"meta": {"set": set_name, "partition": "sealed", "model": "m"}, "rows": rows}


def good_spont(set_name):
    return run(set_name, [row(2, 10, c_edits=1, c_len=50) for _ in range(10)])         # 20% word error, 2% character error, full recall


class Gate(unittest.TestCase):
    def runs(self, read_edits=2, spont_edits=2, recall_hyp=None, loop=False):
        spont = [run(s, [row(spont_edits, 10, c_edits=1, c_len=50, hyp=recall_hyp or "كتاب سړي اوبه", loop=loop and i == 0) for i in range(10)]) for s in sg.SPONTANEOUS]
        return spont + [run(sg.READ, [row(read_edits, 10) for _ in range(10)])]

    def test_all_gates_met_is_a_machine_transcript(self):
        status, checks = sg.evaluate(self.runs())
        self.assertEqual(status, "MACHINE_TRANSCRIPT")
        self.assertTrue(all(ok for _, _, ok in checks))

    def test_a_read_set_over_30_percent_leaves_a_search_lead(self):
        status, _ = sg.evaluate(self.runs(read_edits=4))                                   # 40% on the read set
        self.assertEqual(status, "SEARCH_LEAD")

    def test_low_recall_and_high_error_is_low_resource(self):
        status, _ = sg.evaluate(self.runs(spont_edits=7, recall_hyp="x y z"))               # 70% word error, no word found
        self.assertEqual(status, "LOW_RESOURCE_LANGUAGE")

    def test_one_loop_in_thirty_spontaneous_clips_breaks_the_one_percent_rule(self):
        status, checks = sg.evaluate(self.runs(loop=True))                                  # 1 of 30 = 3.3%
        self.assertEqual(status, "LOW_RESOURCE_LANGUAGE")
        self.assertFalse(dict((n, ok) for n, _, ok in checks)["loops <= 1% of clips (pooled spontaneous)"])

    def test_a_missing_sealed_set_is_not_evaluated(self):
        status, _ = sg.evaluate(self.runs()[:3])
        self.assertEqual(status, "NOT EVALUATED")


if __name__ == "__main__":
    unittest.main()
