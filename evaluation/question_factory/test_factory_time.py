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


if __name__ == "__main__":
    for name, fn in list(globals().items()):
        if name.startswith("test_"):
            fn()
            print("ok", name)
