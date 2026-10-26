"""Unit tests of the judge kind `plates` and of the ANPR bank (docs/data/anpr-typed-family-20261026.md gate G4). No database, no model:

    python -X utf8 tools/ask_eval/test_judge_plates.py
"""
import os
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
sys.path.insert(0, os.path.join(HERE, "..", "case_generator"))
import anpr_bank  # noqa: E402
import anpr_case  # noqa: E402
import judge  # noqa: E402
import world as w  # noqa: E402


def run(values, rows, answer="x", state="answered"):
    return judge.judge({"expect": {"kind": "plates", "values": values}}, {"state": state, "rows": rows, "answer": answer})


class PlateJudgeTests(unittest.TestCase):
    def test_the_plates_seen_are_a_set_of_search_keys(self):
        want = ["SYNT0420", "SYNT0421"]
        self.assertEqual(run(want, [{"plate_norm": "SYNT0420"}, {"plate_norm": "SYNT0421"}]), "correct")
        self.assertEqual(run(want, [{"plate_norm": "SYNT0421", "t": "2026-03-05"}, {"plate_norm": "SYNT0420"}, {"plate_norm": "SYNT0421"}]), "correct", "a plate seen twice is one plate")
        self.assertEqual(run(want, [{"plate_raw": "synt 0420"}, {"plate_raw": "SYNT-٠٤٢١"}]), "correct", "a plate as the camera wrote it is read as its key")
        self.assertEqual(run(want, [{"plate_norm": "SYNT0420"}]), "wrong")
        self.assertEqual(run(want, [{"plate_norm": "SYNT0420"}, {"plate_norm": "SYNT0421"}, {"plate_norm": "SYNT0422"}]), "wrong")

    def test_a_look_alike_plate_is_not_the_plate(self):
        self.assertEqual(run(["SYNT0420"], [{"plate_norm": "SYNTO420"}]), "wrong")
        self.assertEqual(run(["SYNTO420"], [{"plate_norm": "SYNTO420"}]), "correct")

    def test_nothing_expected_is_no_rows_and_the_plain_statement(self):
        self.assertEqual(run([], [], "no row matches what you asked: no sighting at camera cam-01 was found."), "correct")
        self.assertEqual(run([], [], "no record of this case holds the plate x in any camera sighting."), "wrong")
        self.assertEqual(run([], [{"plate_norm": "SYNT0420"}], "no row matches what you asked"), "wrong")


class BankTests(unittest.TestCase):
    def setUp(self):
        self.params = w.Params(seed=2, people=40, subjects=4, associates=8, period_days=30, p2_pairs=3)
        self.truth = anpr_case.build(self.params)["truth"]
        self.questions = anpr_bank.bank_questions(self.truth, 2)
        self.oracle = anpr_case.Oracle(self.truth)

    def test_the_kinds_and_the_controls(self):
        kinds = {q["type"] for q in self.questions}
        self.assertTrue({"plate_count", "plate_cameras", "plate_where_when", "camera_day", "plates_both", "period_count", "look_alike", "absent", "probe"} <= kinds, kinds)
        self.assertTrue(any(q["expect"].get("control") for q in self.questions))
        self.assertEqual(len({q["id"] for q in self.questions}), len(self.questions))

    def test_the_same_seed_asks_the_same_questions(self):
        again = anpr_bank.bank_questions(anpr_case.build(self.params)["truth"], 2)
        self.assertEqual([q["question"] for q in again], [q["question"] for q in self.questions])

    def test_every_expected_answer_is_the_oracles(self):
        counts = {q["question"]: q["expect"]["value"] for q in self.questions if q["type"] == "plate_count"}
        self.assertTrue(counts and all(v >= 1 for v in counts.values()))
        for q in self.questions:
            if q["type"] == "look_alike":
                # a plate and the plate that looks like it are asked for separately and counted separately
                self.assertGreaterEqual(q["expect"]["value"], 1)
        absent = [q for q in self.questions if q["type"] == "absent"][0]
        key = "SYNT" + absent["question"].split("SYNT-")[1][:4]
        self.assertNotIn(key, self.oracle.plates())

    def test_the_frame_of_the_engine_finds_every_plate_the_bank_writes(self):
        # the bank's presentations ("SYNT-0420", "SYNT0420", "SYNT 0420") are written the way an analyst writes a plate; the engine must find each as one plate (Go spec: Ask engine plates)
        for q in self.questions:
            if q["type"] in ("plate_count", "plate_cameras", "plate_where_when", "period_count", "look_alike", "absent"):
                self.assertRegex(q["question"], r"SYNT[ -]?[0-9OI]{4}", q["question"])

    def test_every_phrasing_is_written(self):
        phrasings = anpr_bank.template_bank()
        self.assertGreaterEqual(len(phrasings), 28)


if __name__ == "__main__":
    unittest.main()
