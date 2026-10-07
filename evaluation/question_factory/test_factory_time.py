"""The case clock in the question factory's answer keys and in the replica loader.

The keys are computed by an oracle that reads the RAW payload text (never the stored column), so that the lane is checked
against the evidence and not against itself. A question about the hour, the day or the month means the clock of the place the
case is about: a time with an offset is converted to the case zone, a time without one is already the source's local clock,
which the real ingest reads in that same zone. PostgreSQL does the conversion; nothing here needs a time-zone database.

    python -m pytest evaluation/question_factory/test_factory_time.py
"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import factory as f  # noqa: E402
import load_replica as lr  # noqa: E402

FIELD = {"type": "TIMESTAMP", "source_names": ["timestamp", "Captured At"]}


def test_a_time_with_an_offset_is_converted_to_the_case_zone():
    sql = f.typed_expr(FIELD)
    assert "AT TIME ZONE '%s'" % f.CASE_ZONE in sql
    assert "::timestamptz" in sql
    # explicit-offset text is recognised only when a time part precedes the offset, so a bare date is never mistaken for one
    assert "[T ][0-9]{2}:[0-9]{2}" in sql
    assert "(Z|[+-][0-9]{2}(:?[0-9]{2})?)$" in sql


def test_a_time_without_an_offset_is_taken_as_written():
    sql = f.typed_expr(FIELD)
    assert "THEN (COALESCE(raw_payload->>'timestamp', raw_payload->>'Captured At'))::timestamp" in sql
    assert sql.rstrip().endswith("ELSE NULL END)")


def test_the_oracle_no_longer_reads_a_zulu_time_as_the_local_clock():
    # The old key dropped the Z and kept the UTC clock face: "night" for a UTC-stamped family was 00:00 to 05:59 UTC.
    assert "replace(" not in f.typed_expr(FIELD)


def test_numbers_are_unchanged():
    sql = f.typed_expr({"type": "NUMBER", "source_names": ["duration"]})
    assert "::numeric" in sql and "AT TIME ZONE" not in sql


def test_the_zone_name_cannot_inject_sql():
    # CASE_ZONE is spliced into SQL text; a quote in the setting must not survive.
    assert "'" not in f.CASE_ZONE


def test_the_loader_reads_a_naive_time_in_the_source_zone():
    assert lr.parse_time("2026-04-02 08:10:00") == "2026-04-02 08:10:00 " + lr.SOURCE_ZONE
    assert lr.parse_time("2026-04-02T08:10:00") == "2026-04-02 08:10:00 " + lr.SOURCE_ZONE


def test_the_loader_leaves_a_time_with_an_offset_to_itself():
    assert lr.parse_time("2026-04-02T08:10:00Z") == "2026-04-02T08:10:00+00:00"
    assert lr.parse_time("2026-04-02T08:10:00+05:00") == "2026-04-02T08:10:00+05:00"


def test_the_loader_does_not_invent_a_time():
    assert lr.parse_time("") is None
    assert lr.parse_time("not a time") is None
    assert lr.parse_time(None) is None


def test_a_row_with_no_time_is_marked_so_it_can_be_given_its_ingest_time():
    # In production such a row's canonical time is its ingest time (coalesce(observed_at, ingested_at)); the loader loads it
    # with a marker and the script then sets timestamp = ingested_at, so the stand-in has the same rows.
    assert lr.NO_TIME_MARKER.startswith("1970-01-01")


# ---- stored question files: the keys follow the case clock without regenerating a single question ----------------------------------
RAW = "COALESCE(raw_payload->>'timestamp', raw_payload->>'Captured At')"
# The form every question file generated before the case clock stores for a time field.
OLD = "(CASE WHEN {raw} ~ '^[0-9]{{4}}-[0-9]{{2}}-[0-9]{{2}}' THEN replace({raw},'Z','')::timestamp ELSE NULL END)"
SCOPE = "FROM forensic.records WHERE tenant_id = 'default' AND collection_id = 'case-x' AND record_type = 'anpr'"
SHAPES = {  # the four statement shapes the generator wraps around a time expression
    "night": "SELECT count(*) " + SCOPE + " AND extract(hour from {t}) IN (0,1,2,3,4,5)",
    "earliest": "SELECT min({t})::date " + SCOPE,
    "date_range": "SELECT count(*) " + SCOPE + " AND {t}::date BETWEEN '2026-04-11' AND '2026-04-20'",
    "month_count": "SELECT count(*) " + SCOPE + " AND date_trunc('month', {t}) = '2026-04-01'::timestamp",
}


def old_sql(shape, raw=RAW):
    return SHAPES[shape].format(t=OLD.format(raw=raw))


def test_time_expr_is_what_typed_expr_builds():
    assert f.typed_expr(FIELD) == f.time_expr(f.raw_expr(FIELD))


def test_every_stored_time_shape_becomes_exactly_what_the_generator_now_writes():
    for shape, template in SHAPES.items():
        assert f.rekey_time_sql(old_sql(shape)) == template.format(t=f.time_expr(RAW)), shape
        assert f.OLD_TIME_MARK not in f.rekey_time_sql(old_sql(shape)), shape


def test_two_time_expressions_in_one_statement_are_both_rewritten():
    other = "COALESCE(raw_payload->>'session_start')"
    sql = "SELECT %s - %s %s" % (OLD.format(raw=RAW), OLD.format(raw=other), SCOPE)
    out = f.rekey_time_sql(sql)
    assert out == "SELECT %s - %s %s" % (f.time_expr(RAW), f.time_expr(other), SCOPE)


def test_rekey_is_idempotent():
    once = f.rekey_time_sql(old_sql("night"))
    assert f.rekey_time_sql(once) == once


def test_a_question_with_no_time_in_it_is_left_alone():
    sql = "SELECT count(*) " + SCOPE + " AND raw_payload->>'plate' = 'ABC-123'"
    assert f.rekey_time_sql(sql) == sql
    number = "SELECT avg(" + f.typed_expr({"type": "NUMBER", "source_names": ["duration"]}) + ") " + SCOPE
    assert f.rekey_time_sql(number) == number


def test_a_field_name_with_parentheses_is_still_recognised():
    raw = "COALESCE(raw_payload->>'Time (UTC)', raw_payload->>'timestamp')"
    assert f.rekey_time_sql(old_sql("night", raw)) == SHAPES["night"].format(t=f.time_expr(raw))


def test_rekeying_a_file_refreshes_only_the_time_keys_and_marks_the_file():
    data = {"collection": "case-x", "questions": [
        {"id": "ANPR-night-01", "intent": "night", "kind": "number", "expected": "162", "oracle_sql": old_sql("night")},
        {"id": "ANPR-count_eq-01", "intent": "count_eq", "kind": "number", "expected": "40", "oracle_sql": "SELECT count(*) " + SCOPE}]}
    moved = f.rekey_questions(data, lambda item: "176")
    assert moved == [("ANPR-night-01", "night", "162", "176")]
    assert data["questions"][0]["expected"] == "176" and data["questions"][0]["oracle_sql"] == f.rekey_time_sql(old_sql("night"))
    assert data["questions"][1]["expected"] == "40"  # not a time question: not touched
    assert data["rekeyed_to_case_clock"] == {"zone": f.CASE_ZONE, "questions_rewritten": 1}


def test_a_stored_time_expression_that_is_not_recognised_is_refused_not_left_stale():
    odd = "SELECT count(*) " + SCOPE + " AND extract(hour from replace(raw_payload->>'timestamp','Z','')::timestamp) IN (0,1,2,3,4,5)"
    data = {"collection": "case-x", "questions": [{"id": "ANPR-night-02", "intent": "night", "kind": "number", "expected": "1", "oracle_sql": odd}]}
    try:
        f.rekey_questions(data, lambda item: "0")
    except ValueError as exc:
        assert "ANPR-night-02" in str(exc)
    else:
        raise AssertionError("an expression the rewrite does not recognise must be refused")


def test_saved_time_answers_are_judged_again_against_the_case_clock_keys():
    night = {"id": "ANPR-night-01", "intent": "night", "kind": "number"}
    items = {"ANPR-night-01": night, "ANPR-count_eq-01": {"id": "ANPR-count_eq-01", "intent": "count_eq", "kind": "number"}}
    rows = [
        {"id": "ANPR-night-01", "intent": "night", "kind": "number", "expected": "162", "verdict": "CORRECT", "text": "There were 162 camera sightings at night."},
        {"id": "ANPR-count_eq-01", "intent": "count_eq", "kind": "number", "expected": "40", "verdict": "CORRECT", "text": "40 sightings."},
        {"id": "CDR-night-01", "intent": "night", "kind": "number", "expected": "1283", "verdict": "ERROR", "text": ""},
    ]
    changed = f.rejudge_on_the_case_clock(rows, items, lambda item: "176")
    # the old answer (162) is not the case-clock answer (176): it is now wrong; the question that is not about time is not touched,
    # and a row that never got an answer stays an error
    assert changed == [("ANPR-night-01", "162", "176", "CORRECT", "WRONG")]
    assert rows[0]["expected"] == "176" and rows[0]["verdict"] == "WRONG"
    assert rows[1]["expected"] == "40" and rows[1]["verdict"] == "CORRECT"
    assert rows[2]["verdict"] == "ERROR"
    rows[0]["text"] = "There were 176 camera sightings at night."
    assert f.rejudge_on_the_case_clock(rows, items, lambda item: "176") == [("ANPR-night-01", "176", "176", "WRONG", "CORRECT")]


if __name__ == "__main__":
    for name, fn in list(globals().items()):
        if name.startswith("test_"):
            fn()
            print("ok", name)
