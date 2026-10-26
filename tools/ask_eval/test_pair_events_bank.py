"""Checks of the pair-events bank (Amendment 4): what it asks is a pair of numbers, the right answer is what the two subscribers' own records hold, and a pair whose exports disagree is not asked.

    python -X utf8 tools/ask_eval/test_pair_events_bank.py          # development case 2, about two minutes
"""
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
sys.path.insert(0, os.path.join(HERE, "..", "case_generator"))
sys.path.insert(0, os.path.join(HERE, "..", ".."))
import case_set  # noqa: E402
import pair_events_bank  # noqa: E402


def questions():
    found = [(kind, overrides) for kind, number, overrides in case_set.CASES if number == 2 and kind != "sealed"]
    out, dropped = pair_events_bank.build(case_set.params_for(2, found[0][1]))
    return out, dropped


def test_the_bank_of_development_case_2():
    qs, _ = questions()
    kinds = {q["type"] for q in qs}
    assert kinds == {"pair_sub", "pair_nonsub_first", "pair_month_sub", "pair_control"}, kinds
    for q in qs:
        assert len(re.findall(r"92300\d{7}", q["question"])) == 2, q["question"]
        assert q["expect"]["kind"] == "scalar"
    sub = [q for q in qs if q["type"] == "pair_sub"]
    # the pair the live service answered 8 for: 4 calls, once in each export
    assert any("923000001003" in q["question"] and "923000001007" in q["question"] and q["expect"]["value"] == 4 for q in sub), [q["question"] for q in sub]
    control = [q for q in qs if q["type"] == "pair_control"]
    assert len(control) == 1 and control[0]["expect"]["value"] == 0 and control[0]["expect"].get("empty_is_zero")
    assert all(q["expect"]["value"] >= 2 for q in qs if q["type"] == "pair_month_sub")


def test_the_same_seed_asks_the_same_questions():
    first, _ = questions()
    second, _ = questions()
    assert [q["question"] for q in first] == [q["question"] for q in second]


if __name__ == "__main__":
    for name, fn in sorted(globals().items()):
        if name.startswith("test_") and callable(fn):
            fn()
            print("ok", name)
