"""Checks of the wording gate (Amendment 5): what it accepts and what it refuses, on records written by hand; no case, no model.

    python -X utf8 tools/ask_eval/test_wording_gate.py
"""
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import wording_gate as g  # noqa: E402

A, B = "923001110001", "923001110002"


def q(kind, text, params=None, control=False, probe=False, extra=None):
    expect = {"kind": "probe" if probe else "tuples", "params": params or {}}
    if control:
        expect["control"] = True
        expect["values"] = []
    expect.update(extra or {})
    return {"id": "T-1", "type": kind, "question": text, "expect": expect}


def rec(answer, views, rows=None, state="answered", sql="SELECT x FROM v WHERE slot = 'night' AND shared_device"):
    return {"id": "T-1", "arm": "engine", "state": state, "answer": answer, "views": views, "rows": rows or [], "sql": sql}


def gates(question, record):
    return sorted({gate for gate, _ in g.check_record(question, record)})


def test_a_control_must_say_what_was_searched():
    question = q("anchor_change", "Did %s move house, staying at the new place for at least 3 weeks?" % A, {"anchor_hold_weeks": 3}, control=True)
    ok = "No row matches what you asked: no change of night place that holds for 3 weeks was found for %s." % A
    assert gates(question, rec(ok, ["v_anchor_changes"])) == []
    assert gates(question, rec("No row matches what you asked.", ["v_anchor_changes"])) == ["W2"]
    assert gates(question, rec(ok.replace("3 weeks", "2 weeks"), ["v_anchor_changes"])) == ["W2"]            # not the parameter of the question
    assert gates(question, rec(ok.replace(A, "923001119999"), ["v_anchor_changes"])) == ["W2", "W6"]          # not the number of the question, and a number nobody gave
    assert "W6" in gates(question, rec(ok.replace("3 weeks", "3 weeks and 9 days"), ["v_anchor_changes"]))   # a number nobody gave
    assert gates(question, rec("No row matches what you asked.", ["v_comm_events"])) == []                    # a view the wording does not cover
    assert gates(question, rec("No row matches what you asked.", ["v_anchor_changes"], state="abstained")) == []


def test_the_ratio_of_the_rhythm_view_is_read_as_times():
    question = q("rhythm_change", "Did the daily activity of %s at least double for good?" % A, {"rhythm_ratio_percent": 200}, control=True)
    ok = "No row matches what you asked: no split of the daily count of events of %s has |t| of 5 or more with a larger daily mean at least 2 times the smaller and at least 10 days on each side." % A
    assert gates(question, rec(ok, ["v_rhythm_changes"])) == []


def test_a_score_for_one_pair_is_a_candidate_and_a_conclusion_is_corrected_first():
    row = [{"score": 0.9905}]
    switch = q("probe", "Did %s switch to %s?" % (A, B), probe=True, extra={"caveats": [], "forbid": g.FORBID})
    good = "A candidate, not proof: %s may have taken over from %s; similarity score 0.9905 (a documented formula from 0 to 1, not a probability)." % (B, A)
    assert gates(switch, rec(good, ["v_successor_candidates"], row)) == []
    assert gates(switch, rec("Score: 0.9905.", ["v_successor_candidates"], row)) == ["W11", "W3"]
    same = q("probe", "Is %s the same person as %s?" % (B, A), probe=True, extra={"caveats": [], "forbid": g.FORBID})
    assert gates(same, rec(g.LEAD["successor"] + " " + good, ["v_successor_candidates"], row)) == []
    assert gates(same, rec(good, ["v_successor_candidates"], row)) == ["W4"]
    meet = q("probe", "Did %s and %s meet?" % (A, B), probe=True, extra={"caveats": [], "forbid": g.FORBID})
    assert gates(meet, rec(g.LEAD["meeting"] + " 3 times together at 2 cell sites.", ["v_colocation_windows"], [{"cell_key": "c"}])) == []
    assert gates(meet, rec("3 times together at 2 cell sites.", ["v_colocation_windows"], [{"cell_key": "c"}])) == ["W4"]
    assert gates(meet, rec("Yes, they met.", ["v_colocation_windows"], [{"cell_key": "c"}])) == ["W4", "W5"]


def test_one_speed_alert_states_its_own_figures():
    question = q("speed_alerts", "Which consecutive events of %s imply travel faster than 150 km/h?" % A, {"speed_kmh": 150})
    row = [{"distance_km": 6.29, "speed_kmh": 181.2}]
    ok = "1 speed alert of %s: 6.29 km in 125 s (181.2 km/h) between the events at 2026-03-04 10:12:05 PKT and 2026-03-04 10:14:10 PKT, at different cell sites." % A
    assert gates(question, rec(ok, ["v_speed_alerts"], row)) == []
    assert gates(question, rec("1 row.", ["v_speed_alerts"], row)) == ["W7"]


def test_a_sentence_may_name_only_conditions_the_query_applied():
    question = q("successor_of", "Find the silent number that %s may have replaced." % B, control=True)
    listed = "SELECT old_number, score, shared_device FROM v_successor_candidates WHERE new_number = '%s' ORDER BY score DESC" % B
    applied = "SELECT old_number FROM v_successor_candidates WHERE new_number = '%s' AND shared_device ORDER BY score DESC" % B
    said = "No row matches what you asked: no number that went silent (for at least 7 days) within 14 days before %s started reaches a similarity score of 35 percent and shares its handset." % B
    wrong = dict(rec(said, ["v_successor_candidates"]), sql=listed)
    right = dict(rec(said, ["v_successor_candidates"]), sql=applied)
    assert gates(question, wrong) == ["W8"]
    assert gates(question, right) == []
    night = q("anchor_change", "Did %s move house?" % A, control=True)
    said = "No row matches what you asked: no change of night place that holds for 2 weeks was found for %s." % A
    assert gates(night, dict(rec(said, ["v_anchor_changes"]), sql="SELECT 'night' AS slot FROM v_anchor_changes WHERE number = '%s'" % A)) == ["W8"]
    assert gates(night, dict(rec(said, ["v_anchor_changes"]), sql="SELECT * FROM v_anchor_changes WHERE number = '%s' AND slot = 'night'" % A)) == []


def test_a_move_a_cell_visit_and_a_successor_list_have_their_wording():
    move = q("anchor_change", "Did %s move house?" % A)
    row = [{"first_week_start": "2026-03-16", "distance_km": 8.018}]
    ok = "1 change of night place of %s: from cell site c:2 to cell site c:1, from the week starting 2026-03-16, 8.018 km apart." % A
    assert gates(move, rec(ok, ["v_anchor_changes"], row)) == []
    assert gates(move, rec("To cell key c:1; First week start 2026-03-16; Distance km 8.018.", ["v_anchor_changes"], row)) == ["W9"]
    visit = q("probe", "Where was %s on Tuesday at nine?" % A, probe=True, extra={"caveats": [], "forbid": g.FORBID})
    assert gates(visit, rec("1 cell site.", ["v_cell_visits"], [{"lat": 1}])) == []
    assert gates(visit, rec("1 position.", ["v_cell_visits"], [{"lat": 1}])) == ["W10"]
    cands = q("successor_candidates", "Which numbers may have taken over from %s?" % A)
    assert gates(cands, rec("Candidates, not proof. New number %s; Score 0.69." % B, ["v_successor_candidates"], [{"score": 0.69}])) == []
    assert gates(cands, rec(g.LEAD["successor"] + " Candidates, not proof. x", ["v_successor_candidates"], [{"score": 0.69}])) == []
    assert gates(cands, rec("New number %s; Score 0.69." % B, ["v_successor_candidates"], [{"score": 0.69}])) == ["W11"]


def test_selection_keeps_every_control_and_probe_and_the_first_positive_of_four_kinds():
    questions = [q("coloc_windows", "a"), q("coloc_windows", "b"), q("coloc_windows", "c", control=True), q("probe", "d", probe=True), q("rhythm_change", "e"),
                 q("speed_alerts", "f"), q("speed_alerts", "g"), q("successor_candidates", "h")]
    assert [x["question"] for x in g.select(questions)] == ["a", "c", "d", "f", "h"]


if __name__ == "__main__":
    for name, fn in sorted(globals().items()):
        if name.startswith("test_") and callable(fn):
            fn()
            print("ok", name)
