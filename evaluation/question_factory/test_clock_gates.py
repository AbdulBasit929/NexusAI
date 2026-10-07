"""The run 11 gate checker applies the gates as pre-registered: nothing more, nothing less.

    python -m pytest evaluation/question_factory/test_clock_gates.py     (or run the file: plain test_ functions)
"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import check_clock_gates as g  # noqa: E402

ANSWERED = "state=answered; attempts=1; views=v_anpr; model_ms=1000; exec_ms=10; total_ms=2000"
ABSTAINED = "state=abstained; attempts=2; views=v_anpr"
DECLINED = "state=declined; reason=no view"


def row(rid, intent, verdict, lane=None, family="anpr", text="x", why="", question="a question"):
    r = {"id": rid, "family": family, "intent": intent, "kind": "number", "verdict": verdict, "text": text, "why": why, "question": question}
    if lane:
        r["lane"] = lane
    return r


def test_t1_passes_when_every_night_answer_the_lane_gave_is_right():
    rows = [row("ANPR-night-01", "night", "CORRECT", ANSWERED), row("CDR-night-01", "night", "WRONG", None, "cdr")]  # the second one is the old path's
    assert g.gate_keys(rows, ("night",))[0] == "PASS"


def test_t1_fails_on_one_wrong_lane_answer_and_names_it():
    rows = [row("ANPR-night-01", "night", "CORRECT", ANSWERED), row("CDR-night-01", "night", "WRONG", ANSWERED, "cdr")]
    status, mine, answered, wrong = g.gate_keys(rows, ("night",))
    assert status == "FAIL" and [r["id"] for r in wrong] == ["CDR-night-01"] and len(answered) == 2


def test_a_lane_that_abstained_or_declined_cannot_fail_the_key_gates_but_does_not_pass_them_either():
    rows = [row("ANPR-night-01", "night", "ABSTAINED", ABSTAINED), row("IPDR-night-01", "night", "WRONG", DECLINED, "ipdr")]
    assert g.gate_keys(rows, ("night",))[0] == "NOT ASSESSABLE"


def test_t2_reads_the_other_time_intents_only():
    rows = [row("TRAN-earliest-01", "earliest", "WRONG", ANSWERED, "transaction"), row("ANPR-night-01", "night", "CORRECT", ANSWERED)]
    assert g.gate_keys(rows, ("month_count", "date_range", "earliest", "latest"))[0] == "FAIL"
    assert g.gate_keys(rows, ("night",))[0] == "PASS"


def test_t3_ignores_time_questions_and_sees_a_verdict_or_a_text_change():
    base = [row("A", "count_eq", "CORRECT", text="1"), row("B", "count_eq", "CORRECT", text="2"), row("C", "count_eq", "CORRECT", text="3"), row("N", "night", "WRONG", text="162")]
    same = [row("A", "count_eq", "CORRECT", text="1"), row("B", "count_eq", "CORRECT", text="2"), row("C", "count_eq", "CORRECT", text="3"), row("N", "night", "CORRECT", text="176")]
    assert g.gate_unchanged(same, base) == ("PASS", [], [])
    moved = [row("A", "count_eq", "WRONG", text="1"), row("B", "count_eq", "CORRECT", text="two"), row("C", "count_eq", "CORRECT", text="3")]
    status, verdicts, texts = g.gate_unchanged(moved, base)
    assert status == "FAIL" and verdicts == [("A", "CORRECT", "WRONG")] and texts == ["B"]


def test_t3_cannot_be_read_without_a_baseline():
    assert g.gate_unchanged([row("A", "count_eq", "CORRECT")], None)[0] == "NOT ASSESSABLE"


def test_t4_allows_only_the_recorded_500_and_never_a_timeout():
    recorded = row("ANPR-top_group-01", "top_group", "ERROR", why="HTTP 500", question="Which plate number appears most often in the camera sightings?")
    assert g.gate_errors([recorded], None)[0] == "PASS"
    new500 = row("CDR-avg-01", "avg", "ERROR", why="HTTP 500")
    assert g.gate_errors([recorded, new500], None) == ("FAIL", [("CDR-avg-01", "HTTP 500")])
    assert g.gate_errors([new500], [row("CDR-avg-01", "avg", "ERROR", why="HTTP 500")])[0] == "PASS"  # the same error was already in the baseline
    timeout = row("ANPR-top_group-01", "top_group", "ERROR", why="HTTP 0", question="Which plate number appears most often in the camera sightings?")
    assert g.gate_errors([timeout], None)[0] == "FAIL"


def test_t5_needs_the_clock_named_in_every_lane_answer_to_a_time_question():
    rows = [row("ANPR-night-01", "night", "CORRECT", ANSWERED), row("TRAN-earliest-01", "earliest", "CORRECT", ANSWERED, "transaction"), row("A", "count_eq", "CORRECT", ANSWERED)]
    said = {"ANPR-night-01": ("answered", ["A time of day is read on the case clock (Asia/Karachi, UTC+05:00)."]), "TRAN-earliest-01": ("answered", ["Computed by a model-written query."])}
    status, checked, missing, moved = g.gate_clock_stated(rows, lambda r: said[r["id"]])
    assert status == "FAIL" and checked == ["ANPR-night-01", "TRAN-earliest-01"] and missing == ["TRAN-earliest-01"]  # the count question is not a time question
    said["TRAN-earliest-01"] = ("answered", ["Times are read and shown on the case clock (Asia/Karachi, UTC+05:00)."])
    assert g.gate_clock_stated(rows, lambda r: said[r["id"]])[0] == "PASS"
    assert g.gate_clock_stated(rows, lambda r: ("declined", []))[0] == "NOT ASSESSABLE"


def test_t6_reads_only_anpr_time_answers_the_lane_gave():
    ok = [row("ANPR-night-01", "night", "CORRECT", ANSWERED), row("CDR-night-01", "night", "WRONG", ANSWERED, "cdr")]
    assert g.gate_no_time_rows(ok)[0] == "PASS"
    bad = [row("ANPR-latest-01", "latest", "WRONG", ANSWERED)]
    assert g.gate_no_time_rows(bad)[0] == "FAIL"
    assert g.gate_no_time_rows([row("ANPR-night-01", "night", "ABSTAINED", ABSTAINED)])[0] == "NOT ASSESSABLE"


def test_the_report_names_all_six_gates_in_order():
    rows = [row("ANPR-night-01", "night", "CORRECT", ANSWERED)]
    assert [gate for gate, _, _ in g.report(rows, rows, None)] == ["T1", "T2", "T3", "T4", "T5", "T6"]


if __name__ == "__main__":
    for name, fn in list(globals().items()):
        if name.startswith("test_"):
            fn()
            print("ok", name)
