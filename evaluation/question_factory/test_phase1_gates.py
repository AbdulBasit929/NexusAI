"""The Phase 1 gate checker applies the gates as pre-registered: nothing more, nothing less.

    python -m pytest evaluation/question_factory/test_phase1_gates.py     (or run the file: plain test_ functions)
"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import check_phase1_gates as g  # noqa: E402

ANSWERED = "state=answered; attempts=1; views=v_cdr; model_ms=10000; exec_ms=10; total_ms=11000"
ABSTAINED = "state=abstained; attempts=2; views=v_cdr"


def row(rid, verdict, intent="count_all", lane=None, text="Number of records: 5.", seconds=10.0):
    r = {"id": rid, "family": "cdr", "intent": intent, "verdict": verdict, "text": text, "seconds": seconds, "expected": "5"}
    if lane:
        r["lane"] = lane
    return r


def arm(*rows):
    return {r["id"]: r for r in rows}


def many(prefix, n, verdict, **kw):
    return [row("%s-%03d" % (prefix, i), verdict, **kw) for i in range(n)]


# ---- S1: the thresholds are the pre-registered ones

def test_thresholds_are_the_numbers_phase_1_fixed_in_advance():
    multimodal_a = arm(*many("c", 34, "CORRECT"), *many("w", 21, "WRONG"), *many("a", 24, "ABSTAINED"))   # 79 questions
    demo_a = arm(*many("c", 42, "CORRECT"), *many("w", 18, "WRONG"), *many("a", 22, "ABSTAINED"))          # 82 questions
    assert g.thresholds(multimodal_a) == (46, 7)
    assert g.thresholds(demo_a) == (55, 8)


def test_s1_passes_and_fails_on_each_condition():
    a = arm(*many("c", 34, "CORRECT"), *many("w", 21, "WRONG"), *many("a", 24, "ABSTAINED"))
    good = arm(*many("c", 71, "CORRECT"), *many("w", 3, "WRONG"), *many("a", 5, "ABSTAINED", text="I did not run this question, because I could not apply everything it asks."))
    assert g.gate_s1(a, good)["pass"]
    too_wrong = arm(*many("c", 68, "CORRECT"), *many("w", 8, "WRONG"), *many("a", 3, "ABSTAINED", text="I could not apply it."))
    assert not g.gate_s1(a, too_wrong)["pass"]
    too_few = arm(*many("c", 45, "CORRECT"), *many("w", 3, "WRONG"), *many("a", 31, "ABSTAINED", text="I could not apply it."))
    assert not g.gate_s1(a, too_few)["pass"]
    silent = arm(*many("c", 71, "CORRECT"), *many("w", 3, "WRONG"), row("a-0", "ABSTAINED", text="no"), *many("a", 4, "ABSTAINED", text="I could not apply it."))
    result = g.gate_s1(a, silent)
    assert not result["pass"] and result["abstentions_without_reason"] == ["a-0"]


# ---- S2 and the run 14 reading of it: nothing right gets worse

def test_s2_names_every_move_and_allows_three_correct_to_abstained():
    prev = arm(row("x", "CORRECT"), row("y", "CORRECT"), row("z", "WRONG"), row("w", "ABSTAINED"))
    cur = arm(row("x", "CORRECT"), row("y", "ABSTAINED"), row("z", "CORRECT"), row("w", "WRONG"))
    result = g.gate_s2(prev, cur)
    assert result["correct_to_abstained"] == ["y"] and result["gained"] == 1 and result["other"] == ["w"] and result["pass"]
    worse = arm(row("x", "WRONG"), row("y", "CORRECT"), row("z", "WRONG"), row("w", "ABSTAINED"))
    assert not g.gate_s2(prev, worse)["pass"]
    four = g.gate_s2(arm(*many("q", 4, "CORRECT")), arm(*many("q", 4, "ABSTAINED")))
    assert len(four["correct_to_abstained"]) == 4 and not four["pass"]


# ---- S3, S4, the reading note

def test_s3_wants_the_name_and_absent_value_questions_answered_or_abstained_never_wrong():
    ok = arm(row("n", "ABSTAINED", "unbound_name"), row("a", "CORRECT", "absent_value"), row("o", "WRONG", "count_all"))
    assert g.gate_s3(ok)["pass"] and g.gate_s3(ok)["questions"] == 2
    bad = arm(row("n", "WRONG", "unbound_name"))
    assert g.gate_s3(bad) == {"pass": False, "questions": 1, "not_correct_or_abstained": ["n"]}


def test_s4_is_any_verdict_that_is_not_correct_wrong_or_abstained():
    assert g.gate_s4(arm(row("a", "CORRECT"), row("b", "WRONG"), row("c", "ABSTAINED")))["pass"]
    assert g.gate_s4(arm(row("a", "ERROR")))["errors"] == {"a": "ERROR"}


def test_an_abstention_the_judge_scored_correct_is_named():
    b = arm(row("a", "CORRECT", lane=ABSTAINED, text="I did not run this question ... status on \"200\" ..."), row("b", "CORRECT", lane=ANSWERED))
    assert g.credited_abstentions(b) == ["a"]


def test_timing_reports_the_lane_answers_and_their_model_time():
    b = arm(row("a", "CORRECT", lane=ANSWERED, seconds=12.0), row("b", "CORRECT", lane=ANSWERED, seconds=20.0), row("c", "ABSTAINED", lane=ABSTAINED, seconds=5.0))
    t = g.timing(b)
    assert t["all"]["n"] == 3 and t["lane_answered"]["n"] == 2 and t["lane_answered"]["max"] == 20.0 and t["lane_model"]["median"] == 10.0


# ---- the corpus: number and text checks, and an abstention never passes

def crow(rid, check, expect, text, route=("records_sql",), stated=None, http=200):
    return {"id": rid, "check": check, "expect": expect, "answer_text": text, "route": list(route), "stated": stated, "http": http, "question": "q " + rid}


def test_a_text_check_needs_every_expected_string_and_ignores_commas():
    r = crow("x", "contains", ["Incoming", "2906", "Outgoing", "2592"], "Data: 3,142; Incoming: 2,906; Outgoing: 2,592")
    assert g.score(r) is True
    assert g.score(crow("x", "contains", ["Incoming", "2906"], "Total calls: 5,500.")) is False
    assert g.score(crow("x", "top", "923461678183", "Msisdn: 923461678183; Number of calls: 529.")) is True
    assert g.score(crow("x", "top", ["a", "b"], "only b is here")) is True
    assert g.score(crow("x", "number", 5, "five", stated=True)) is True and g.score(crow("x", "number", 5, "five", stated=False)) is False
    assert g.score(crow("x", "qualitative", None, "an overview")) is None and g.score(crow("x", "empty", None, "none found")) is None


def test_an_abstention_never_passes_even_when_it_quotes_the_expected_value():
    quoting = crow("p2", "contains", ["video"], "I did not run this question, because ... \"video_embedded_audio_transcript_segment\" ...", route=("governed_sql", "clarification"))
    assert g.score(quoting) is False
    withheld = crow("w", "contains", ["video"], "You asked about video, which I have not searched.", route=("verified_only_withheld",))
    assert g.score(withheld) is False


def test_the_corpus_comparison_lists_losses_gains_and_the_lane_answers_whose_check_fails():
    off = {"a": crow("a", "contains", ["x"], "x", ("records_sql",)), "b": crow("b", "contains", ["y"], "no", ("records_sql",)), "c": crow("c", "contains", ["z"], "z", ("records_sql",)),
           "d": crow("d", "number", 5, "five", stated=True)}
    on = {"a": crow("a", "contains", ["x"], "Total: 1", ("governed_sql",)), "b": crow("b", "contains", ["y"], "y", ("governed_sql",)), "c": crow("c", "contains", ["z"], "z", ("records_sql",)),
          "d": crow("d", "number", 5, "No record contains it", ("governed_sql",), stated=False)}
    result = g.compare_corpus(off, on)
    assert result["lost"] == ["a", "d"] and result["gained"] == ["b"] and result["both_ok"] == ["c"]
    assert result["lane_answered_failed"] == ["a", "d"]
    assert sorted(result["changed"]) == ["a", "b", "d"] and result["errors"] == []
    off["c"]["http"] = 500
    assert g.compare_corpus(off, on)["errors"] == ["c"]


# ---- a fresh-seed set has no stage 1 arm: the existing path is the baseline

def test_without_a_stage_1_arm_the_existing_path_is_the_baseline_of_s2(capsys=None):
    import contextlib
    import io
    a = arm(*many("c", 34, "CORRECT"), *many("w", 21, "WRONG"), *many("a", 24, "ABSTAINED"))
    good = arm(*many("c", 71, "CORRECT"), *many("w", 3, "WRONG"), *many("a", 5, "ABSTAINED", text="I did not run this question, because I could not apply it."))
    out = io.StringIO()
    with contextlib.redirect_stdout(out):
        code = g.report_arms(a, good)
    text = out.getvalue()
    assert code == 0 and "S2 not worse (vs the existing path) PASS" in text and "stage 1" not in text.split("S2")[0]
    # a question the existing path answered right and the lane first answers wrongly fails S2
    worse = dict(good)
    worse["c-000"] = row("c-000", "WRONG")
    out = io.StringIO()
    with contextlib.redirect_stdout(out):
        code = g.report_arms(a, worse)
    assert code == 1 and "S2 not worse (vs the existing path) FAIL" in out.getvalue()


if __name__ == "__main__":
    failed = 0
    for name, fn in sorted(globals().items()):
        if name.startswith("test_") and callable(fn):
            try:
                fn()
                print("ok   " + name)
            except AssertionError as exc:  # noqa: PERF203
                failed += 1
                print("FAIL " + name + " " + str(exc))
    sys.exit(1 if failed else 0)
