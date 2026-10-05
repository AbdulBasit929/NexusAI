"""Checks of the factory's verdict logic, run with: python -m pytest evaluation/question_factory/test_factory_judge.py (or plain python)."""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import factory as f  # noqa: E402


def case(kind, expected, text, route=None, status=200, **extra):
    item = dict({"kind": kind}, **extra)
    return f.judge(item, expected, text, status, route)[0]


def test_numbers():
    assert case("number", "8642", "There are 8,642 CDR records in this case.") == "CORRECT"
    assert case("number", "5", "There are 6 records.") == "WRONG"
    assert case("number", "32425.000000", "The average is 32,425.") == "CORRECT"
    assert case("number", "72.6519752", "The average is 72.65.") == "CORRECT"
    assert case("number", "5", "I can only state this from a plan I can verify.", ["verified_only_withheld"]) == "ABSTAINED"
    assert case("number", "5", "", status=200) == "NOT_STATED"
    assert case("number", "5", "x", status=500) == "ERROR"


def test_the_confident_wrong_class():
    # The defect that started this work: a total returned for a question that names something it cannot bind.
    assert case("unbound", None, "There are 8,642 CDR records in this case.", ["records_sql"], total=8642) == "WRONG"
    assert case("unbound", None, "I could not tie that name to any number. Which number should I use?", ["clarification"], total=8642) == "ABSTAINED"
    assert case("absence", None, "There are 0 CDR records for that number.", ["records_sql"], total=8642) == "CORRECT"
    assert case("absence", None, "There are 8,642 CDR records in this case.", ["records_sql"], total=8642) == "WRONG"
    assert case("absence", None, "No matching records were found.", ["records_sql"], total=8642) == "CORRECT"


def test_top_and_date():
    item = {"kind": "top", "winners": ["923001110002"], "others": ["923001110001"]}
    assert f.judge(item, None, "The most common is 923001110002 with 421 records.", 200, [])[0] == "CORRECT"
    assert f.judge(item, None, "The most common is 923001110001.", 200, [])[0] == "WRONG"
    assert case("date", "2026-04-02", "The earliest was on April 2, 2026.") == "CORRECT"
    assert case("date", "2026-04-02", "The earliest was 2026-04-02 00:00:02.") == "CORRECT"
    assert case("date", "2026-04-02", "The earliest was May 3, 2026.") == "WRONG"


def test_tags_and_disclosure():
    item = {"kind": "number", "total": 8642}
    verdict, why = f.judge(item, "10", "Showing 20 of 8,642 CDR records that matched this question.", 200, [])
    assert verdict == "WRONG" and why.startswith("rows listed instead of an answer")
    verdict, why = f.judge(item, "10", "There are no CDR records involving 923451112233 in this case.", 200, [])
    assert verdict == "WRONG" and why.startswith("absence claimed")
    verdict, why = f.judge(item, "10", "There are 8,642 CDR records in this case.", 200, [])
    assert verdict == "WRONG" and why.startswith("total returned")
    verdict, why = f.judge(item, "3905649.9", "The average network volume across IPDR sessions is 0.", 200, [])
    assert verdict == "WRONG" and why.startswith("zero returned")
    assert f.judge(item, "0", 'I did not apply the condition "between 2025" to a curated field, so this result counts every record of its kind rather than only those you asked for: 5.', 200, [])[0] == "ABSTAINED"


if __name__ == "__main__":
    for name, fn in list(globals().items()):
        if name.startswith("test_"):
            fn()
            print("ok", name)
