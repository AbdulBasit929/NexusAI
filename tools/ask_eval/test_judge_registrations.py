"""Unit tests of the judge kinds `sims`, `statuses`, `models` and of the subscriber bank (docs/data/subscriber-device-family-20261028.md gate G4). No database, no model:

    python -X utf8 tools/ask_eval/test_judge_registrations.py
"""
import os
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
sys.path.insert(0, os.path.join(HERE, "..", "case_generator"))
import case_set  # noqa: E402
import judge  # noqa: E402
import subscriber_bank  # noqa: E402


def run(kind, values, rows, answer="x", state="answered", **extra):
    return judge.judge({"expect": dict({"kind": kind, "values": values}, **extra)}, {"state": state, "rows": rows, "answer": answer})


class JudgeTests(unittest.TestCase):
    def test_sims_are_the_imsi_column_as_a_set(self):
        want = ["001010000001000", "001010000801000"]
        self.assertEqual(run("sims", want, [{"imsi": "001010000001000", "status": "active"}, {"imsi": "001010000801000"}]), "correct")
        self.assertEqual(run("sims", want, [{"imsi": "001010000001000"}, {"imsi": "001010000801000"}, {"imsi": "001010000801000"}]), "correct", "a SIM seen in two rows is one SIM")
        self.assertEqual(run("sims", want, [{"imsi": "001010000001000"}]), "wrong")
        self.assertEqual(run("sims", want, [{"msisdn": "923000001000"}]), "wrong", "an answer with no IMSI column is not the IMSIs")
        self.assertEqual(run("sims", want, [], state="declined"), "declined")

    def test_statuses_are_the_status_of_the_registrations_in_force(self):
        self.assertEqual(run("statuses", ["active", "deactivated"], [{"status": "deactivated"}, {"status": "active"}]), "correct")
        self.assertEqual(run("statuses", ["active"], [{"status": "active"}, {"status": "active"}]), "correct")
        self.assertEqual(run("statuses", ["active"], [{"status": "active"}, {"status": "suspended"}]), "wrong")
        # nothing in force: no rows and the plain statement, never "no record of this case"
        self.assertEqual(run("statuses", [], [], answer="no row matches what you asked: no registration of 923000001000 was found in the files of this case."), "correct")
        self.assertEqual(run("statuses", [], [], answer="no record of this case holds it"), "wrong")
        self.assertEqual(run("statuses", [], [{"status": "active"}], answer="no row matches"), "wrong")

    def test_models_are_the_model_column_as_a_set(self):
        self.assertEqual(run("models", ["SYNTH PHONE 01, Model SP01, Global, 2025"], [{"model": "SYNTH PHONE 01, Model SP01, Global, 2025", "tac_listed": True}]), "correct")
        self.assertEqual(run("models", ["A"], [{"model": "B"}]), "wrong")
        self.assertEqual(run("models", [], [{"model": None}]), "correct", "a handset the list does not hold has no model")

    def test_numbers_are_a_set_and_a_sim_serial_is_not_a_number(self):
        rows = [{"msisdn": "923000001000", "iccid": "8900000000000010002", "imsi": "001010000001000"}]
        self.assertEqual(run("set", ["923000001000"], rows), "correct")
        self.assertEqual(run("set", ["923000001000", "923000001001"], rows), "wrong")


class BankTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        table = {number: overrides for kind, number, overrides in case_set.CASES if kind == "development"}
        cls.truth, cls.events = subscriber_bank.build_truth(case_set.params_for(2, table[2]))
        cls.questions = subscriber_bank.bank_questions(cls.truth, cls.events, 2)

    def test_the_bank_has_every_kind_and_the_controls(self):
        kinds = {q["type"] for q in self.questions}
        self.assertEqual(kinds, {"numbers_of_sim", "sims_of_number", "in_force", "after_end_count", "after_end_numbers", "unregistered", "conflicts", "handset_model", "handset_count", "makers",
                                 "absent", "probe"})
        self.assertGreaterEqual(len(self.questions), 18)
        self.assertGreaterEqual(sum(1 for q in self.questions if q["expect"].get("control")), 3)

    def test_the_expected_values_are_the_oracles(self):
        by = {}
        for q in self.questions:
            by.setdefault(q["type"], []).append(q)
        self.assertEqual(by["after_end_count"][0]["expect"]["value"], len(by["after_end_numbers"][0]["expect"]["values"]))
        self.assertGreaterEqual(by["after_end_count"][0]["expect"]["value"], 1)
        self.assertEqual(len(by["conflicts"][0]["expect"]["values"]), 2)
        self.assertEqual(len(by["unregistered"][0]["expect"]["values"]), 2)
        swapped = [q for q in by["sims_of_number"] if len(q["expect"]["values"]) == 2]
        self.assertTrue(swapped, "a SIM swap has two IMSIs")
        for q in by["probe"]:
            self.assertEqual(q["expect"]["kind"], "probe")

    def test_the_bank_is_deterministic_and_the_phrasings_differ_by_seed(self):
        again = subscriber_bank.bank_questions(self.truth, self.events, 2)
        self.assertEqual(self.questions, again)
        other = subscriber_bank.bank_questions(self.truth, self.events, 3)
        self.assertNotEqual([q["question"] for q in self.questions], [q["question"] for q in other])

    def test_every_phrasing_is_written_once_in_the_template_bank(self):
        templates = subscriber_bank.template_bank()
        self.assertEqual(len(templates), 4 * 11 + 2)


if __name__ == "__main__":
    unittest.main()
