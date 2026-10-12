#!/usr/bin/env python3
"""Compare what a real file contains with what the pipeline loaded, on aggregates only. Only PASS or FAIL leaves the machine.

    # 1. profile the real file locally (keeps exact counts; the report stays in the vault)
    python -I -X utf8 tools/real_data/profile_structure.py FILE --mode local --out PROFILE.json
    # 2. print the aggregate query for the loaded rows, run it with psql, keep the single JSON line it returns
    python -I -X utf8 tools/real_data/reconcile_aggregates.py emit-sql --table comm_event --tenant T --collection C \
        --evidence-id UUID --tz Asia/Karachi > aggregates.sql
    psql -At -f aggregates.sql > LOADED.json
    # 3. compare, with a small hand-written map from file columns to typed columns
    python -I -X utf8 tools/real_data/reconcile_aggregates.py compare --profile PROFILE.json --loaded LOADED.json \
        --map MAP.json --quarantined 12 --duplicates 0

The comparison checks, per the acceptance rule for a layout (roadmap section 4.4): the row-accounting identity
(rows read = loaded + quarantined + duplicates), distinct counts and empty counts per mapped column, the local-time minimum and
maximum, and the histograms of enumerations and of number classes after the map. It prints counts and canonical labels, never a
value from the file. The time zone is applied by PostgreSQL in the emitted query (AT TIME ZONE), so this tool needs no tz data.

MAP.json (illustrative, no real values):
  {"table": "comm_event",
   "columns": [{"source": "<file column>", "target": "a_raw", "distinct": "equal|at_most", "nulls": "equal|at_least"}],
   "time": {"source": "<file column>", "target": "start_utc"},
   "histograms": [{"source": "<file column>", "target": "direction", "enum_map": {"<file label>": "<canonical label>"}}],
   "classes": [{"source": "<file column>", "target": "a_kind", "class_map": {"92_prefix_12_digits": "person_like"}}]}
"""
import argparse
import json
import os
import re
import sys

TABLE_SPECS = {
    "comm_event": {
        "time": "start_utc", "id": "event_id",
        "columns": ["a_raw", "a_norm", "b_raw", "b_norm", "imsi", "imei", "tac", "cell_key", "lac_tac", "cell_raw"],
        "hist": ["event_type", "direction", "a_kind", "b_kind", "dataset_kind"],
    },
    "data_session": {
        "time": "start_utc", "id": "session_id",
        "columns": ["msisdn", "imsi", "imei", "ue_ip", "public_ip", "server_ip", "apn", "app", "host", "cell_key"],
        "hist": ["protocol", "rat", "app_category"],
    },
    "app_usage": {
        "time": "start_utc", "id": "usage_id",
        "columns": ["msisdn", "imsi", "imei", "terminal", "apn", "app", "province", "city"],
        "hist": ["app_category"],
    },
    "plate_sighting": {
        "time": "observed_utc", "id": "sighting_id",
        "columns": ["plate_raw", "plate_norm", "camera_id", "province"],
        "hist": ["source_kind", "review_state"],
    },
}
IDENT = re.compile(r"^[a-z_][a-z0-9_]*$")
UUID = re.compile(r"^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")
TZ = re.compile(r"^[A-Za-z0-9_+\-/]+$")


def sql_literal(value):
    return "'" + str(value).replace("'", "''") + "'"


def emit_sql(table, tenant, collection, evidence_id, tz):
    if table not in TABLE_SPECS:
        raise SystemExit("unknown table %r; choose one of %s" % (table, ", ".join(sorted(TABLE_SPECS))))
    if not UUID.match(evidence_id):
        raise SystemExit("evidence id must be a UUID")
    if not TZ.match(tz):
        raise SystemExit("time zone name has unexpected characters")
    spec = TABLE_SPECS[table]
    for name in spec["columns"] + spec["hist"] + [spec["time"]]:
        assert IDENT.match(name)
    where = "tenant_id = %s AND collection_id = %s AND evidence_id = %s" % (
        sql_literal(tenant), sql_literal(collection), sql_literal(evidence_id))
    rel = "forensic.%s" % table
    distinct = ", ".join("%s, count(DISTINCT %s)" % (sql_literal(c), c) for c in spec["columns"])
    nulls = ", ".join("%s, count(*) FILTER (WHERE %s IS NULL)" % (sql_literal(c), c) for c in spec["columns"])
    hists = ", ".join(
        "%s, (SELECT coalesce(jsonb_object_agg(k, n), '{}'::jsonb) FROM (SELECT coalesce(%s::text, '<null>') AS k, count(*) AS n "
        "FROM %s WHERE %s GROUP BY 1) h)" % (sql_literal(c), c, rel, where) for c in spec["hist"])
    return (
        "-- Aggregates of the loaded rows for one evidence item. Read-only. Run with: psql -At -f this.sql > LOADED.json\n"
        "SELECT jsonb_build_object(\n"
        "  'table', %s,\n  'rows', count(*),\n  'distinct', jsonb_build_object(%s),\n  'nulls', jsonb_build_object(%s),\n"
        "  'time_local_min', to_char(min(%s) AT TIME ZONE %s, 'YYYY-MM-DD HH24:MI:SS'),\n"
        "  'time_local_max', to_char(max(%s) AT TIME ZONE %s, 'YYYY-MM-DD HH24:MI:SS'),\n"
        "  'histograms', jsonb_build_object(%s)\n"
        ")::text\nFROM %s WHERE %s;\n"
    ) % (sql_literal(table), distinct, nulls, spec["time"], sql_literal(tz), spec["time"], sql_literal(tz), hists, rel, where)


class Report(object):
    def __init__(self):
        self.lines = []
        self.failed = 0

    def check(self, ok, metric, expected, loaded, note=""):
        if not ok:
            self.failed += 1
        self.lines.append("%-5s %-44s expected=%-12s loaded=%-12s %s" % ("PASS" if ok else "FAIL", metric, expected, loaded, note))

    def text(self):
        verdict = "RECONCILE=PASS" if not self.failed else "RECONCILE=FAIL (%d checks)" % self.failed
        return "\n".join(self.lines + [verdict])


def norm_time(text):
    if text is None:
        return None
    text = str(text).replace("T", " ").strip()
    return re.sub(r"\.0+$", "", text)


def column_of(sheet, name):
    for col in sheet["columns"]:
        if col["header"] == name:
            return col
    return None


def compare(profile, loaded, mapping, quarantined, duplicates, sheet_index=0):
    if profile.get("mode") != "local":
        raise SystemExit("the profile must be a local-mode report (exact counts); shareable reports are not enough")
    sheet = profile["sheets"][sheet_index]
    rep = Report()
    rows_read = sheet["row_count"]
    rep.check(rows_read == loaded["rows"] + quarantined + duplicates, "row accounting (read = loaded + quarantined + duplicate)",
              rows_read, "%d+%d+%d" % (loaded["rows"], quarantined, duplicates))

    for spec in mapping.get("columns", []):
        src = column_of(sheet, spec["source"])
        tgt = spec["target"]
        if src is None:
            rep.check(False, "column %s present in file" % spec["source"], "present", "missing")
            continue
        file_empty = rows_read - src["nonempty"]
        want = spec.get("distinct", "equal")
        got = loaded["distinct"].get(tgt)
        if got is None:
            rep.check(False, "distinct %s -> %s" % (spec["source"], tgt), src["distinct"], "not loaded")
        else:
            if src.get("distinct_is_lower_bound"):
                rep.check(got >= src["distinct"], "distinct %s -> %s (file count is a lower bound)" % (spec["source"], tgt), ">=%d" % src["distinct"], got)
            elif want == "at_most":
                rep.check(got <= src["distinct"], "distinct %s -> %s (normalisation may merge)" % (spec["source"], tgt), "<=%d" % src["distinct"], got)
            else:
                rep.check(got == src["distinct"], "distinct %s -> %s" % (spec["source"], tgt), src["distinct"], got)
        null_rule = spec.get("nulls", "equal")
        nulls = loaded["nulls"].get(tgt)
        if nulls is not None:
            ok = nulls >= file_empty if null_rule == "at_least" else nulls == file_empty
            rep.check(ok, "empty %s -> %s" % (spec["source"], tgt), file_empty if null_rule == "equal" else ">=%d" % file_empty, nulls)

    time_spec = mapping.get("time")
    if time_spec:
        src = column_of(sheet, time_spec["source"])
        stats = (src or {}).get("time", {})
        if not stats.get("min"):
            rep.check(False, "time column %s parsed" % time_spec["source"], "min/max", stats.get("day_month_order", "none"),
                      "day and month order may be ambiguous: resolve it in the layout first")
        else:
            rep.check(norm_time(stats["min"]) == norm_time(loaded.get("time_local_min")), "time minimum (local clock)", "file", "table", "compared as text")
            rep.check(norm_time(stats["max"]) == norm_time(loaded.get("time_local_max")), "time maximum (local clock)", "file", "table", "compared as text")

    for spec in mapping.get("histograms", []):
        src = column_of(sheet, spec["source"])
        file_hist = {}
        for label, n in (src or {}).get("vocabulary", {}).items():
            canon = spec["enum_map"].get(label, "<unmapped:%s>" % label)
            file_hist[canon] = file_hist.get(canon, 0) + n
        got = loaded["histograms"].get(spec["target"], {})
        for label in sorted(set(file_hist) | set(got)):
            rep.check(file_hist.get(label, 0) == got.get(label, 0), "histogram %s = %s" % (spec["target"], label),
                      file_hist.get(label, 0), got.get(label, 0))

    for spec in mapping.get("classes", []):
        src = column_of(sheet, spec["source"])
        file_hist = {}
        for label, n in (src or {}).get("value_classes", {}).items():
            canon = spec["class_map"].get(label, "unknown")
            file_hist[canon] = file_hist.get(canon, 0) + n
        got = loaded["histograms"].get(spec["target"], {})
        for label in sorted(set(file_hist) | set(got)):
            rep.check(file_hist.get(label, 0) == got.get(label, 0), "class %s = %s" % (spec["target"], label),
                      file_hist.get(label, 0), got.get(label, 0))
    return rep


def main(argv=None):
    ap = argparse.ArgumentParser(description="Reconcile a real file's aggregates with what the pipeline loaded.")
    sub = ap.add_subparsers(dest="command", required=True)
    e = sub.add_parser("emit-sql", help="print the aggregate query for loaded rows")
    e.add_argument("--table", required=True)
    e.add_argument("--tenant", required=True)
    e.add_argument("--collection", required=True)
    e.add_argument("--evidence-id", required=True)
    e.add_argument("--tz", default="Asia/Karachi", help="the layout's source time zone (applied by PostgreSQL)")
    c = sub.add_parser("compare", help="compare a local-mode profile with the loaded aggregates")
    c.add_argument("--profile", required=True)
    c.add_argument("--loaded", required=True)
    c.add_argument("--map", required=True)
    c.add_argument("--quarantined", type=int, default=0)
    c.add_argument("--duplicates", type=int, default=0)
    c.add_argument("--sheet", type=int, default=0)
    args = ap.parse_args(argv)

    if args.command == "emit-sql":
        sys.stdout.write(emit_sql(args.table, args.tenant, args.collection, args.evidence_id, args.tz))
        return 0
    with open(args.profile, "r", encoding="utf-8") as handle:
        profile = json.load(handle)
    with open(args.loaded, "r", encoding="utf-8") as handle:
        loaded = json.loads(handle.read().strip().splitlines()[-1])
    with open(args.map, "r", encoding="utf-8") as handle:
        mapping = json.load(handle)
    rep = compare(profile, loaded, mapping, args.quarantined, args.duplicates, args.sheet)
    print(rep.text())
    return 1 if rep.failed else 0


if __name__ == "__main__":
    sys.exit(main())
