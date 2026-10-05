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


def test_lane_summary_and_absence():
    # an honest absence after a search is a correct answer to "does this exist"
    item = {"kind": "absence", "total": 5005}
    assert f.judge(item, None, "No record in this case contains 923000000001: I searched every identifier field of every evidence family in this case.", 200, ["governed_sql"])[0] == "CORRECT"
    assert f.judge(item, None, "The total is 5,005.", 200, [])[0] == "WRONG"
    rows = [
        {"verdict": "CORRECT", "lane": "state=answered; attempts=1; views=v_cdr; model_ms=41000; exec_ms=12; total_ms=41100"},
        {"verdict": "WRONG", "lane": "state=answered; attempts=2; views=v_cdr; model_ms=82000; exec_ms=15; total_ms=82200"},
        {"verdict": "ABSTAINED", "lane": "state=abstained; attempts=2; views=v_cdr; model_ms=80000; exec_ms=0; total_ms=80050"},
        {"verdict": "WRONG", "lane": "state=declined; attempts=1; views=; model_ms=0; exec_ms=0; total_ms=5; reason=model: TIMEOUT_OR_UNAVAILABLE"},
        {"verdict": "CORRECT"},
    ]
    text = f.lane_summary(rows)
    assert "4 of 5 responses carry the header" in text
    assert "answered" in text and "CORRECT 1, WRONG 1" in text
    assert "two attempts: 1" in text
    assert "TIMEOUT_OR_UNAVAILABLE" in text
    assert f.lane_summary([{"verdict": "CORRECT"}]) == ""


if __name__ == "__main__":
    for name, fn in list(globals().items()):
        if name.startswith("test_"):
            fn()
            print("ok", name)
