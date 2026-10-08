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


# ---- run 12: T3 read as "nothing gets worse", and part B, the lane-first spot check of the time questions ----
def test_t3b_nothing_gets_worse_lets_an_improvement_through_and_ignores_the_time_questions():
    previous = [row("A", "count_eq", "CORRECT"), row("B", "count_eq", "ABSTAINED"), row("N", "night", "CORRECT")]
    now = [row("A", "count_eq", "CORRECT"), row("B", "count_eq", "CORRECT"), row("N", "night", "WRONG")]
    assert g.gate_not_worse(now, previous) == ("PASS", [], [("B", "ABSTAINED", "CORRECT")])
    now[0]["verdict"] = "ABSTAINED"
    status, worse, _ = g.gate_not_worse(now, previous)
    assert status == "FAIL" and worse == [("A", "CORRECT", "ABSTAINED")]
    assert g.gate_not_worse(now, None)[0] == "NOT ASSESSABLE"


def test_b2_a_time_question_that_was_right_must_stay_right():
    previous = [row("N1", "night", "CORRECT"), row("E1", "earliest", "WRONG"), row("A", "count_eq", "CORRECT")]
    now = [row("N1", "night", "WRONG"), row("E1", "earliest", "CORRECT"), row("A", "count_eq", "WRONG")]
    assert g.gate_time_not_worse(now, previous) == ("FAIL", [("N1", "CORRECT", "WRONG")])  # the count question is not a time question
    now[0]["verdict"] = "CORRECT"
    assert g.gate_time_not_worse(now, previous)[0] == "PASS"
    assert g.gate_time_not_worse(now, None)[0] == "NOT ASSESSABLE"


def test_b_p1_counts_the_existing_paths_wrong_time_answers_that_are_right_now():
    previous = [row("N1", "night", "WRONG"), row("L1", "latest", "WRONG"), row("N2", "night", "WRONG", ANSWERED),
                row("C", "count_eq", "WRONG"), row("E", "earliest", "CORRECT")]
    now = [row("N1", "night", "CORRECT", ANSWERED), row("L1", "latest", "WRONG"), row("N2", "night", "CORRECT", ANSWERED),
           row("C", "count_eq", "CORRECT"), row("E", "earliest", "CORRECT")]
    before, fixed = g.prediction_fixed(now, previous)
    assert before == ["N1", "L1"]  # N2 was the lane's own wrong answer and C is not a time question
    assert fixed == ["N1"]


def test_the_spot_report_names_its_gates_and_the_report_adds_t3b_only_with_a_previous_arm():
    rows = [row("N1", "night", "CORRECT", ANSWERED)]
    assert [gate for gate, _, _ in g.report_spot(rows, rows)] == ["B1", "B2", "B3", "B-P1"]
    assert [gate for gate, _, _ in g.report_spot(rows, None)] == ["B1", "B2", "B3"]
    assert "T3b" not in [gate for gate, _, _ in g.report(rows, rows, None)]
    assert "T3b" in [gate for gate, _, _ in g.report(rows, rows, None, rows)]


def test_a_lane_first_answer_that_differs_from_its_key_fails_b1():
    rows = [row("N1", "night", "WRONG", ANSWERED), row("E1", "earliest", "CORRECT", ANSWERED)]
    status = {gate: s for gate, s, _ in g.report_spot(rows, [row("N1", "night", "WRONG"), row("E1", "earliest", "WRONG")])}
    assert status["B1"] == "FAIL" and status["B2"] == "PASS"


if __name__ == "__main__":
    for name, fn in list(globals().items()):
        if name.startswith("test_"):
            fn()
            print("ok", name)
