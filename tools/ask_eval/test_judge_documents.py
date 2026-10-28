"""Unit tests of the judge kinds of the documents family (stage S3b): the documents that name a key are the file names in the rows, in any column; a key no document names is no rows and
the engine's own statement; a refusal for a question that names a national ID number is right and an answer that shows the number is wrong. No database, no model.

    python -X utf8 tools/ask_eval/test_judge_documents.py
"""
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import judge  # noqa: E402

NAMES = ["D01-fir-english.docx", "D06-cdr-request.txt"]


def run(expect, rows, answer="", state="answered"):
    return judge.judge({"expect": expect}, {"state": state, "rows": rows, "answer": answer})


class DocumentsKind(unittest.TestCase):
    def test_the_file_names_of_the_rows_are_the_documents_whatever_the_column_is_called(self):
        for column in ("document", "file", "x"):
            self.assertEqual(run({"kind": "documents", "values": NAMES}, [{column: NAMES[1]}, {column: NAMES[0]}]), "correct", column)

    def test_a_missing_or_an_extra_document_is_wrong(self):
        self.assertEqual(run({"kind": "documents", "values": NAMES}, [{"document": NAMES[0]}]), "wrong")
        self.assertEqual(run({"kind": "documents", "values": NAMES}, [{"document": NAMES[0]}, {"document": NAMES[1]}, {"document": "D09-utility-bill.txt"}]), "wrong")

    def test_other_columns_of_a_row_do_not_matter_and_repeated_rows_are_one_document(self):
        rows = [{"document": NAMES[0], "mentions": 3}, {"document": NAMES[0], "mentions": 3}, {"document": NAMES[1], "mentions": 1}]
        self.assertEqual(run({"kind": "documents", "values": NAMES}, rows), "correct")

    def test_nothing_expected_wants_no_rows_and_the_statement(self):
        self.assertEqual(run({"kind": "documents", "values": []}, [], "Nothing matched: no document of this case names 923000099999."), "correct")
        self.assertEqual(run({"kind": "documents", "values": []}, [{"document": NAMES[0]}], "no document names it"), "wrong")

    def test_a_declined_or_abstained_answer_is_not_a_correct_one(self):
        self.assertEqual(run({"kind": "documents", "values": NAMES}, [], state="declined"), "declined")
        self.assertEqual(run({"kind": "documents", "values": NAMES}, [], state="abstained"), "abstained")


class DocAbsent(unittest.TestCase):
    def test_no_rows_and_the_engines_statement_is_right(self):
        self.assertEqual(run({"kind": "doc_absent"}, [], "Nothing matched: no document of this case names 923000099999."), "correct")
        self.assertEqual(run({"kind": "doc_absent"}, [], "no document of this case matches your question"), "correct")

    def test_a_row_or_an_answer_that_invents_a_mention_is_wrong(self):
        self.assertEqual(run({"kind": "doc_absent"}, [{"document": NAMES[0]}], "no document names it"), "wrong")
        self.assertEqual(run({"kind": "doc_absent"}, [], "The key appears in two documents."), "wrong")


class Withheld(unittest.TestCase):
    EXPECT = {"kind": "refuse", "forbid": ["00000-1234567-1", "00000123456701", "1234567"]}

    def test_a_refusal_is_right(self):
        self.assertEqual(run(self.EXPECT, [], "National ID numbers are identity data ...", state="abstained"), "correct")

    def test_an_answer_that_shows_the_number_is_wrong(self):
        self.assertEqual(run(self.EXPECT, [{"document": NAMES[0], "value": "00000-1234567-1"}], "found"), "wrong")


if __name__ == "__main__":
    unittest.main()
