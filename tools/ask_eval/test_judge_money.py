"""Unit tests of the judge kinds of the transactions family: an amount is right to the paisa whether the rows give it as a number or as text; rows of accounts, counts and amounts must be exactly the
expected rows; an account or wallet the statements do not hold is no rows and the engine's own statement. No database, no model.

    python -X utf8 tools/ask_eval/test_judge_money.py
"""
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import judge  # noqa: E402

A, B = "PK00SYNT0000000000001001", "PK00SYNT0000000000001002"


def run(expect, rows, answer="", state="answered"):
    return judge.judge({"expect": expect}, {"state": state, "rows": rows, "answer": answer})


class Money(unittest.TestCase):
    def test_an_amount_as_a_number_or_as_text_to_the_paisa(self):
        expect = {"kind": "money", "value": "1234.50"}
        for value in (1234.5, "1234.50", 1234.50, "1234.5"):
            self.assertEqual(run(expect, [{"total": value}]), "correct", value)

    def test_another_amount_more_than_a_paisa_off_or_more_than_one_row_is_wrong(self):
        expect = {"kind": "money", "value": "1234.50"}
        self.assertEqual(run(expect, [{"total": "1234.52"}]), "wrong")
        self.assertEqual(run(expect, [{"total": 1234.5}, {"total": 1234.5}]), "wrong")
        self.assertEqual(run(expect, []), "wrong")

    def test_a_true_value_in_another_column_still_counts_and_a_boolean_is_not_an_amount(self):
        self.assertEqual(run({"kind": "money", "value": "1.00"}, [{"n": True}]), "wrong")
        self.assertEqual(run({"kind": "money", "value": "500000.00"}, [{"account": A, "total": "500000.00"}]), "correct")


class MoneyTuples(unittest.TestCase):
    EXPECT = {"kind": "money_tuples", "values": [[A, {"m": "100.00"}, 3], [B, {"m": "50.50"}, 1]]}

    def test_the_rows_are_the_expected_rows_in_any_order_with_extra_columns(self):
        rows = [{"account": B, "transactions": 1, "total_amount": "50.50", "x": "y"}, {"account": A, "transactions": 3, "total_amount": 100.0}]
        self.assertEqual(run(self.EXPECT, rows), "correct")

    def test_a_wrong_amount_a_wrong_count_a_missing_or_an_extra_row_is_wrong(self):
        good = [{"account": A, "transactions": 3, "total_amount": "100.00"}, {"account": B, "transactions": 1, "total_amount": "50.50"}]
        self.assertEqual(run(self.EXPECT, good[:1]), "wrong")
        self.assertEqual(run(self.EXPECT, good + [{"account": "PK00SYNT0000000000001003", "transactions": 1, "total_amount": "9.00"}]), "wrong")
        self.assertEqual(run(self.EXPECT, [dict(good[0], transactions=2), good[1]]), "wrong")
        self.assertEqual(run(self.EXPECT, [dict(good[0], total_amount="100.02"), good[1]]), "wrong")

    def test_nothing_expected_wants_no_rows(self):
        self.assertEqual(run({"kind": "money_tuples", "values": []}, []), "correct")
        self.assertEqual(run({"kind": "money_tuples", "values": []}, [{"account": A}]), "wrong")


class TxnAbsent(unittest.TestCase):
    def test_no_rows_and_the_engines_statement_is_right(self):
        self.assertEqual(run({"kind": "txn_absent"}, [], "Nothing matched: no transaction of PK00SYNT0000000999999 was found in the files of this case."), "correct")
        self.assertEqual(run({"kind": "txn_absent"}, [], "no transaction of the wallet or number 923000099999 was found in the files of this case"), "correct")

    def test_the_one_row_of_a_zero_count_is_right_only_with_the_engines_statement(self):
        self.assertEqual(run({"kind": "txn_absent"}, [{"n": 0}], "no transaction found"), "correct")
        self.assertEqual(run({"kind": "txn_absent"}, [{"n": 0}], "N: 0"), "wrong")

    def test_a_row_with_a_figure_or_an_invented_figure_is_wrong(self):
        self.assertEqual(run({"kind": "txn_absent"}, [{"n": 3}], "no transaction found"), "wrong")
        self.assertEqual(run({"kind": "txn_absent"}, [], "The account has 12 transactions."), "wrong")


if __name__ == "__main__":
    unittest.main()
