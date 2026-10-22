"""Tests of the stage-3b/3c question builder (stage3bc_bank.py): the shape of a bank, the controls, the probes, that the same seed gives the same bank, and that every written phrasing is
well formed. No database, no model. One development case is built (about 20 seconds):

    python -X utf8 tools/ask_eval/test_stage3bc_bank.py
"""
import json
import os
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
sys.path.insert(0, os.path.join(HERE, "..", "case_generator"))
import case_set  # noqa: E402
import judge  # noqa: E402
import stage3bc_bank as bank  # noqa: E402


def development_world(seed):
    overrides = next(o for kind, number, o in case_set.CASES if number == seed and kind == "development")
    return bank.World(case_set.params_for(seed, overrides))


class Templates(unittest.TestCase):
    def test_every_phrasing_is_well_formed(self):
        questions = bank.template_bank()
        self.assertGreater(len(questions), 60)
        for q in questions:
            self.assertNotIn("%", q["question"].replace("%)", ""), q["question"])   # no placeholder left (a score of "70%" is not written by a template)
            self.assertNotIn("None", q["question"])
            self.assertTrue(q["question"].endswith("?") or q["question"].endswith(".") or q["question"].endswith(")"), q["question"])
            self.assertIn(q["expect"]["kind"], ("tuples", "probe"))
        ids = [q["id"] for q in questions]
        self.assertEqual(len(ids), len(set(ids)))

    def test_every_template_of_a_kind_names_what_its_parameters_need(self):
        by_kind = {}
        for q in bank.template_bank():
            by_kind.setdefault(q["type"], []).append(q)
        for q in by_kind["silent_numbers"]:
            self.assertIn("10 days", q["question"])
        for q in by_kind["speed_alerts"]:
            self.assertIn("150 km/h", q["question"])
        for q in by_kind["anchor_week"]:
            self.assertIn("2026-03-16", q["question"])


class OneCase(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.world = development_world(2)
        cls.b = bank.bank_3b(cls.world, 2)
        cls.c = bank.bank_3c(cls.world, 2)

    def test_the_shape_of_each_bank(self):
        for questions, kinds in ((self.b, {"silent_numbers": 1, "successor_candidates": 3, "successor_of": 2, "successor_device": 2, "probe": 2}),
                                 (self.c, {"anchor_change": 2, "speed_alerts": 2, "anchor_week": 1, "rhythm_change": 3, "probe": 2})):
            counted = {}
            for q in questions:
                counted[q["type"]] = counted.get(q["type"], 0) + 1
            self.assertEqual(counted, kinds)
            self.assertEqual(len(questions), 10)
            self.assertEqual(sum(1 for q in questions if q["expect"].get("control")), 3)

    def test_controls_expect_nothing_and_the_others_something(self):
        for q in self.b + self.c:
            if q["expect"]["kind"] != "tuples":
                continue
            if q["expect"].get("control"):
                self.assertEqual(q["expect"]["values"], [], q["question"])
            elif q["type"] != "successor_device":
                self.assertTrue(q["expect"]["values"], q["question"])

    def test_probes_carry_the_fixed_caveat_and_phrases(self):
        probes = [q for q in self.b + self.c if q["type"] == "probe"]
        self.assertEqual(len(probes), 4)
        caveats = {c for q in probes for c in q["expect"]["caveats"]}
        self.assertEqual(caveats, {"behavioural similarity, not identity", "cell sites, not positions of a person", "a change in the pattern of use, not an explanation"})
        for q in probes:
            self.assertEqual(q["expect"]["forbid"], ["they met", "met with", "same person", "is also", "confirmed"])

    def test_a_bank_names_a_parameter_of_its_own(self):
        named_b = {n for q in self.b for n in q["expect"]["params"]}
        named_c = {n for q in self.c for n in q["expect"]["params"]}
        self.assertIn("silence_days", named_b)
        self.assertTrue({"speed_kmh"} <= named_c)

    def test_the_same_seed_gives_the_same_bank(self):
        again = bank.bank_3b(development_world(2), 2), bank.bank_3c(development_world(2), 2)
        self.assertEqual(json.dumps(again[0]), json.dumps(self.b))
        self.assertEqual(json.dumps(again[1]), json.dumps(self.c))

    def test_the_values_are_those_the_judge_reads(self):
        # a returned row holding the expected values of a positive question is judged correct, and one with a different number wrong
        q = next(x for x in self.b if x["type"] == "silent_numbers")
        rows = [{"number": number, "last_event_utc": when} for number, when in q["expect"]["values"]]
        self.assertEqual(judge.judge(q, {"state": "answered", "rows": rows, "answer": ""}), "correct")
        wrong = [dict(rows[0], number="923000009999")] + rows[1:]
        self.assertEqual(judge.judge(q, {"state": "answered", "rows": wrong, "answer": ""}), "wrong")


if __name__ == "__main__":
    unittest.main()
