#!/usr/bin/env python3
"""Check the licence ledger (roadmap principle P7: no model, dataset or library is fetched or shipped without a ledger entry).

    python -X utf8 tools/licence_ledger/check_ledger.py                 # check docs/compliance/licence-ledger.csv against the repository
    python -X utf8 tools/licence_ledger/check_ledger.py --summary       # also print counts and the ship-blocking rows

What it enforces (standard library only):
  schema      the exact columns, unique ids, known kind / usage / evidence tier / status, a real check date, evidence and licence filled in
  status      "approved" needs evidence tier A (read from the artifact itself) and a permissive licence with no restrictive token;
              conditional, blocked and unverified rows must say what the condition is; blocked and unverified rows are ship-blocking
  coverage    every package pinned in ingestion/forensic_records/requirements.txt has a row (config_ref "pkg:<name>"), and every model,
              detector or OCR setting in docker-compose.forensic-records.yaml has a row naming it in config_ref (the variable name)
It cannot tell whether a stated licence is true. Tier A means the licence was read from the artifact (package metadata, a LICENSE file);
tier B means it was reported by a project page or search and the file still has to be opened; tier C means unknown.
"""
import argparse
import csv
import datetime
import os
import re
import sys

COLUMNS = ["id", "component", "kind", "role", "version", "usage", "licence", "evidence_tier", "evidence", "restrictions", "status",
           "ship_blocking", "condition", "config_ref", "checked_on"]
KINDS = {"library", "model", "dataset", "infrastructure", "system-package"}
USAGES = {"running", "configured", "candidate", "evaluation-only"}
TIERS = {"A", "B", "C"}
STATUSES = {"approved", "conditional", "blocked", "unverified"}
PERMISSIVE = re.compile(r"\b(MIT|BSD|Apache|ISC|PostgreSQL|CC0|HPND|Python)\b", re.I)
RESTRICTIVE = re.compile(r"AGPL|(?<!L)GPL|\bNC\b|non-?commercial|research-only|share-?alike|CC[- ]BY-SA|unknown|unresolved", re.I)
# Compose settings that name a model, detector, OCR engine or weights location.
MODEL_SETTING = re.compile(r"^\s+(FORENSIC_[A-Z0-9_]*(?:MODEL|DETECTOR_PATH|TESSERACT_PATH)[A-Z0-9_]*):", re.M)

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
LEDGER = os.path.join(REPO, "docs", "compliance", "licence-ledger.csv")
REQUIREMENTS = os.path.join(REPO, "ingestion", "forensic_records", "requirements.txt")
COMPOSE = os.path.join(REPO, "docker-compose.forensic-records.yaml")


def read_rows(path):
    with open(path, encoding="utf-8", newline="") as handle:
        reader = csv.DictReader(handle)
        return list(reader.fieldnames or []), list(reader)


def package_names(requirements_path):
    names = []
    with open(requirements_path, encoding="utf-8") as handle:
        for line in handle:
            text = line.split("#", 1)[0].strip()
            if not text or text.startswith("-"):
                continue
            name = re.split(r"[=<>!~;\[ ]", text, 1)[0]
            if name:
                names.append(re.sub(r"[-_.]+", "-", name).lower())
    return names


def refs_of(rows):
    refs = set()
    for row in rows:
        for part in (row.get("config_ref") or "").split(";"):
            if part.strip():
                refs.add(part.strip())
    return refs


def check(ledger_path=LEDGER, requirements_path=REQUIREMENTS, compose_path=COMPOSE):
    """Returns a list of problems (empty = pass)."""
    problems = []
    columns, rows = read_rows(ledger_path)
    if columns != COLUMNS:
        return ["columns are %s, expected %s" % (columns, COLUMNS)]
    seen = set()
    for n, row in enumerate(rows, start=2):
        label = "row %d (%s)" % (n, row["id"] or "no id")
        if not row["id"]:
            problems.append("%s: id is empty" % label)
        elif row["id"] in seen:
            problems.append("%s: duplicate id" % label)
        seen.add(row["id"])
        for key in ("component", "role", "licence", "evidence", "version"):
            if not row[key].strip():
                problems.append("%s: %s is empty" % (label, key))
        for key, allowed in (("kind", KINDS), ("usage", USAGES), ("evidence_tier", TIERS), ("status", STATUSES)):
            if row[key] not in allowed:
                problems.append("%s: %s %r is not one of %s" % (label, key, row[key], sorted(allowed)))
        try:
            datetime.date.fromisoformat(row["checked_on"])
        except ValueError:
            problems.append("%s: checked_on %r is not a date" % (label, row["checked_on"]))
        if row["status"] == "approved":
            if row["evidence_tier"] != "A":
                problems.append("%s: approved needs evidence tier A (the licence read from the artifact)" % label)
            if not PERMISSIVE.search(row["licence"]) or RESTRICTIVE.search(row["licence"]):
                problems.append("%s: approved but the licence %r is not plainly permissive" % (label, row["licence"]))
        elif not row["condition"].strip():
            problems.append("%s: %s needs a condition" % (label, row["status"]))
        blocking = row["status"] in ("blocked", "unverified")
        if row["ship_blocking"] not in ("yes", "no") or (row["ship_blocking"] == "yes") != blocking:
            problems.append("%s: ship_blocking must be yes exactly for blocked and unverified rows" % label)
    refs = refs_of(rows)
    for name in package_names(requirements_path):
        if "pkg:" + name not in refs:
            problems.append("coverage: %s is pinned in requirements.txt and has no ledger row (config_ref pkg:%s)" % (name, name))
    with open(compose_path, encoding="utf-8") as handle:
        settings = sorted(set(MODEL_SETTING.findall(handle.read())))
    for setting in settings:
        if setting not in refs:
            problems.append("coverage: %s is set in the compose file and no ledger row names it in config_ref" % setting)
    return problems


def summary(rows):
    lines = []
    by_status = {}
    for row in rows:
        by_status[row["status"]] = by_status.get(row["status"], 0) + 1
    lines.append("rows: %d  (%s)" % (len(rows), ", ".join("%s %d" % (k, by_status[k]) for k in sorted(by_status))))
    for usage in ("running", "configured"):
        bad = [r for r in rows if r["usage"] == usage and r["ship_blocking"] == "yes"]
        lines.append("%s and ship-blocking: %d" % (usage, len(bad)))
        for r in bad:
            lines.append("  %s  %s  [%s]  %s" % (r["id"], r["component"], r["status"], r["condition"]))
    return lines


def main(argv=None):
    ap = argparse.ArgumentParser(description="Check the licence ledger against the repository.")
    ap.add_argument("--ledger", default=LEDGER)
    ap.add_argument("--requirements", default=REQUIREMENTS)
    ap.add_argument("--compose", default=COMPOSE)
    ap.add_argument("--summary", action="store_true")
    args = ap.parse_args(argv)
    problems = check(args.ledger, args.requirements, args.compose)
    for p in problems:
        print("FAIL  " + p)
    if args.summary:
        _, rows = read_rows(args.ledger)
        for line in summary(rows):
            print(line)
    print("LEDGER=%s problems=%d" % ("FAIL" if problems else "PASS", len(problems)))
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
