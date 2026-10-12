#!/usr/bin/env python3
"""Confirm a layout entry against a structure report of a real file, by structure only.

    python -I -X utf8 tools/real_data/profile_structure.py FILE --out REPORT.json            # shareable (default) or --mode local
    python -I -X utf8 tools/real_data/confirm_layout.py --layout call-c4-telenor-style --report REPORT.json

Checks (names, labels and formats only; no value is read or printed):
  container       kind, encoding and delimiter are the ones the entry declares
  header row      where the header sits, against the entry's fixed index or scan window
  header names    the report's header cells equal the entry's columns, in order (duplicates included)
  time columns    every time format the report saw is one the entry accepts; day or month first is reported
  vocabulary      every label seen in an enumerated column is mapped by the entry (an unmapped label is a failure)
  number classes  the classes seen in the party columns (informational)
It prints PASS, FAIL or INFO per check and CONFIRM=PASS or CONFIRM=FAIL. A pass makes the header list and the vocabulary
confirmed; it does not confirm the unconfirmed items that need documentation or an analyst (time zone, direction codes ...).
"""
import argparse
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.abspath(os.path.join(HERE, "..", "..")))
from ingestion.forensic_records.layout_registry import load_registry  # noqa: E402
from ingestion.forensic_records.layout_registry.matching import normalize_header  # noqa: E402
from ingestion.forensic_records.layout_registry.spec import column_ref  # noqa: E402

PROFILE_FORMATS = {
    "excel_serial_1900": {"excel_serial"},
    "iso_naive": {"iso_naive", "iso_with_offset"},
    "slash_24h_seconds": {"dmy_or_mdy_24h"},
    "slash_24h_minutes": {"dmy_or_mdy_24h"},
    "slash_12h_seconds": {"dmy_or_mdy_12h"},
    "dot_dmy_24h_seconds": {"dmy_or_mdy_24h"},
    "slash_date": {"dmy_or_mdy_date"},
}
KIND_TO_PROFILE = {"xlsx": "xlsx", "spreadsheetml": "spreadsheetml", "csv": "text", "tsv": "text"}
DELIMITER_NAMES = {",": "comma", "\t": "tab", ";": "semicolon", "|": "pipe"}


class Result(object):
    def __init__(self):
        self.lines = []
        self.failed = 0

    def add(self, state, check, detail=""):
        if state == "FAIL":
            self.failed += 1
        self.lines.append("%-4s %-16s %s" % (state, check, detail))

    def text(self):
        return "\n".join(self.lines + ["CONFIRM=PASS" if not self.failed else "CONFIRM=FAIL (%d checks)" % self.failed])


def confirm(entry, report, sheet_index=0):
    result = Result()
    file_info = report["file"]
    sheet = report["sheets"][sheet_index]
    container = entry["container"]

    wanted = {KIND_TO_PROFILE[k] for k in container["kinds"]}
    result.add("PASS" if file_info["kind"] in wanted else "FAIL", "container",
               "kind %s (entry: %s)" % (file_info["kind"], ", ".join(container["kinds"])))
    if file_info["kind"] == "text":
        if container.get("encoding"):
            same = (file_info.get("encoding") or "").lower() == container["encoding"].lower()
            result.add("PASS" if same else "FAIL", "encoding", "%s (entry: %s)" % (file_info.get("encoding"), container["encoding"]))
        if container.get("delimiter"):
            same = file_info.get("delimiter") == DELIMITER_NAMES.get(container["delimiter"])
            result.add("PASS" if same else "FAIL", "delimiter", "%s (entry: %s)" % (file_info.get("delimiter"), DELIMITER_NAMES.get(container["delimiter"])))

    row = container["header_row"]
    at = sheet.get("header_row_index")
    if at is None:
        result.add("FAIL", "header row", "the profile found no header row")
    elif row["mode"] == "fixed":
        result.add("PASS" if at == row["index"] else "FAIL", "header row", "at %s (entry: fixed %s)" % (at, row["index"]))
    else:
        result.add("PASS" if at < row["max_rows"] else "FAIL", "header row", "at %s (entry: scan within %s rows)" % (at, row["max_rows"]))

    file_headers = [c["header"] for c in sheet["columns"]]
    entry_headers = [c["name"] for c in entry["header"]["columns"]]
    aliases = {column_ref(n): alts for n, alts in (entry["header"].get("aliases") or {}).items()}
    expected_norm = [normalize_header(n) for n in entry_headers]
    actual_norm = [normalize_header(h) for h in file_headers]
    if actual_norm == expected_norm:
        result.add("PASS", "header names", "%d columns equal the entry, in order" % len(entry_headers))
    else:
        accepted = []
        occurrence = {}
        for name in entry_headers:
            occurrence[name] = occurrence.get(name, 0) + 1
            accepted.append({normalize_header(name)} | {normalize_header(a) for a in aliases.get((name, occurrence[name]), [])})
        if len(actual_norm) == len(accepted) and all(a in acc for a, acc in zip(actual_norm, accepted)):
            result.add("PASS", "header names", "%d columns equal the entry in order, some through declared aliases" % len(entry_headers))
        else:
            missing = [n for n in entry_headers if normalize_header(n) not in actual_norm]
            extra = [h for h in file_headers if normalize_header(h) not in expected_norm]
            result.add("FAIL", "header names", "file has %d columns, entry %d; missing from the file: %s; not in the entry: %s" % (
                len(file_headers), len(entry_headers), missing or "none", extra or "none"))

    by_norm = {}
    for column in sheet["columns"]:
        by_norm.setdefault(normalize_header(column["header"]), []).append(column)

    time_spec = entry.get("time") or {}
    allowed = set()
    for fmt in time_spec.get("formats", []):
        allowed |= PROFILE_FORMATS.get(fmt, set())
    for name, ref in (time_spec.get("columns") or {}).items():
        column_name, occ = column_ref(ref)
        matches = by_norm.get(normalize_header(column_name), [])
        column = matches[occ - 1] if len(matches) >= occ else None
        if column is None or "time" not in column:
            result.add("FAIL", "time " + name, "column %s has no time information in the report" % column_name)
            continue
        seen = set(column["time"]["formats"])
        order = column["time"].get("day_month_order", "not_applicable")
        extra = sorted(seen - allowed)
        detail = "formats seen: %s" % ", ".join(sorted(seen))
        if order in ("day_first", "month_first", "ambiguous"):
            detail += "; day and month order: %s (entry: %s)" % (order, time_spec.get("date_order"))
        result.add("PASS" if not extra else "FAIL", "time " + name, detail + ("" if not extra else "; not accepted by the entry: %s" % extra))

    for index, rule in enumerate(entry.get("enums") or []):
        column_name, occ = column_ref(rule["column"])
        matches = by_norm.get(normalize_header(column_name), [])
        column = matches[occ - 1] if len(matches) >= occ else None
        if column is None or "vocabulary" not in column:
            result.add("INFO", "vocabulary", "%s: no vocabulary in the report (more than 25 values, or not an allowed enumeration column)" % column_name)
            continue
        known = set()
        for r in entry["enums"]:
            if column_ref(r["column"]) == (column_name, occ):
                known |= {k.casefold() for k in r["map"]}
        seen = set(column["vocabulary"])
        unmapped = sorted(label for label in seen if label.casefold() not in known and not (label.endswith(".0") and label[:-2].casefold() in known))
        if index and any(column_ref(r["column"]) == (column_name, occ) for r in entry["enums"][:index]):
            continue
        result.add("PASS" if not unmapped else "FAIL", "vocabulary", "%s: %d labels seen%s" % (
            column_name, len(seen), "" if not unmapped else "; not mapped by the entry: %s" % unmapped))

    for field in ("a", "b", "msisdn"):
        spec = (entry.get("fields") or {}).get(field)
        names = []
        if isinstance(spec, dict) and "by_direction" in spec:
            names = list(spec["by_direction"].values())
        elif spec is not None:
            names = [spec["column"] if isinstance(spec, dict) else spec]
        for name in names:
            column_name, occ = column_ref(name)
            matches = by_norm.get(normalize_header(column_name), [])
            column = matches[occ - 1] if len(matches) >= occ else None
            if column and "value_classes" in column:
                top = sorted(column["value_classes"].items(), key=lambda kv: -kv[1])[:4]
                result.add("INFO", "number classes", "%s: %s" % (column_name, ", ".join("%s %s" % kv for kv in top)))
    return result


def main(argv=None):
    ap = argparse.ArgumentParser(description="Confirm a layout entry against a structure report (no values).")
    ap.add_argument("--layout", required=True)
    ap.add_argument("--report", required=True)
    ap.add_argument("--sheet", type=int, default=0)
    args = ap.parse_args(argv)
    entry = load_registry().get(args.layout)
    with open(args.report, "r", encoding="utf-8") as handle:
        report = json.load(handle)
    result = confirm(entry, report, args.sheet)
    print(result.text())
    return 1 if result.failed else 0


if __name__ == "__main__":
    sys.exit(main())
