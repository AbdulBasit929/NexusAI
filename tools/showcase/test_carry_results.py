"""Tests of tools/showcase/carry_results.py: an earlier answer is kept only for a question that has not changed.

    python -I -X utf8 tools/showcase/test_carry_results.py
"""
import json
import os
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)

import carry_results as cr  # noqa: E402


def q(qid, text, value):
    return {"id": qid, "question": text, "expect": {"kind": "scalar", "value": value}}


OLD = {x["id"]: x for x in [q("A", "How many calls?", 5), q("B", "How many evidence files?", 90), q("C", "How many texts?", 3), q("D", "Old only?", 1)]}
NEW = {x["id"]: x for x in [q("A", "How many calls?", 5), q("B", "How many evidence files?", 95), q("C", "How many text messages?", 3), q("E", "A new one?", 2)]}
RESULTS = {i: {"id": i, "state": "answered"} for i in ("A", "B", "C", "D")}


class Carry(unittest.TestCase):
    def test_only_an_unchanged_question_keeps_its_answer(self):
        carried, changed, gone = cr.carry(OLD, NEW, RESULTS)
        self.assertEqual([r["id"] for r in carried], ["A"])
        self.assertEqual(changed, ["B", "C"])
        self.assertEqual(gone, ["D"])

    def test_a_result_for_a_question_the_old_bank_never_had_is_dropped(self):
        carried, changed, gone = cr.carry(OLD, NEW, {"E": {"id": "E", "state": "answered"}})
        self.assertEqual(carried, [])
        self.assertEqual(changed, ["E"])

    def test_the_command_writes_the_carried_lines_and_later_files_win(self):
        with tempfile.TemporaryDirectory() as tmp:
            paths = {n: os.path.join(tmp, n) for n in ("old.json", "new.json", "r1.jsonl", "r2.jsonl", "out.jsonl")}
            json.dump({"questions": list(OLD.values())}, open(paths["old.json"], "w", encoding="utf-8"))
            json.dump({"questions": list(NEW.values())}, open(paths["new.json"], "w", encoding="utf-8"))
            open(paths["r1.jsonl"], "w").write(json.dumps({"id": "A", "state": "declined"}) + "\n")
            open(paths["r2.jsonl"], "w").write(json.dumps({"id": "A", "state": "answered"}) + "\n")
            self.assertEqual(cr.main([paths["old.json"], paths["new.json"], paths["r1.jsonl"], paths["r2.jsonl"], "--out", paths["out.jsonl"]]), 0)
            lines = [json.loads(line) for line in open(paths["out.jsonl"], encoding="utf-8")]
            self.assertEqual(lines, [{"id": "A", "state": "answered"}])


if __name__ == "__main__":
    unittest.main()
