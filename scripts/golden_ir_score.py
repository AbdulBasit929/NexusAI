#!/usr/bin/env python3
"""Golden-IR scorer — measures the PLAN the generator produced, not the answer.

Answer accuracy conflates four different things: which family was chosen, which
fields were selected, which aggregate was applied, and whether execution and
presentation worked. When the generator was unstable we could see only the last
one, so "it returned 9,200 instead of 75,000" was all we knew. This scores the
generated plan directly against hand-written gold plans, and attributes every
failure to one cause.

Input is a live-eval output directory produced with FORENSIC_IR_SHADOW=true, so
the plans are the ones the real production generator emitted.

Gold plans: scripts/ir_spike/gold_plans.json — written by hand from the question
text before any model was called, and verified field-by-field against the
production semantic layer.

Usage:
  python scripts/golden_ir_score.py <eval-out-dir> [--json out.json]
"""
import argparse
import collections
import json
import os
import sys

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
GOLD = os.path.join(REPO, "scripts", "ir_spike", "gold_plans.json")


def norm_measures(plan):
    return sorted(((m.get("op") or "").upper(), m.get("field_id") or "")
                  for m in (plan.get("measures") or []))


def norm_filters(plan):
    """Compare only what the executor reads: `values` for BETWEEN/IN, `value`
    otherwise. Comparing both scored a correct BETWEEN as wrong."""
    out = []
    for f in plan.get("filters") or []:
        op = (f.get("op") or "").upper()
        field = f.get("field_id") or ""
        if op in ("IS_NULL", "IS_NOT_NULL"):
            out.append((field, op, "", ()))
        elif op == "BETWEEN":
            out.append((field, op, "", tuple(v.strip().lower() for v in (f.get("values") or []))))
        elif op == "IN":
            out.append((field, op, "", tuple(sorted(v.strip().lower() for v in (f.get("values") or [])))))
        else:
            out.append((field, op, (f.get("value") or "").strip().lower(), ()))
    return sorted(out)


def fields_used(plan):
    used = set(plan.get("project") or []) | set(plan.get("group_fields") or [])
    for f in plan.get("filters") or []:
        if f.get("field_id"):
            used.add(f["field_id"])
    for m in plan.get("measures") or []:
        if m.get("field_id"):
            used.add(m["field_id"])
    return used


def count_field_only_diff(got, gold):
    """True when the ONLY measure difference is COUNT(field) where gold counts
    rows. COUNT(x) excludes NULLs, so this is not equivalent in general — but
    measured 2026-09-22 the counted fields (cdr.call_type, ipdr.session_id,
    ipdr.protocol) have ZERO nulls in the case, so these plans execute to the
    same number. Reported as its own class rather than silently credited."""
    g, d = norm_measures(got), norm_measures(gold)
    if len(g) != len(d) or not g:
        return False
    if sorted(op for op, _ in g) != sorted(op for op, _ in d):
        return False
    differs = False
    for (gop, gf), (dop, df) in zip(g, d):
        if gop != dop:
            return False
        if gf == df:
            continue
        if gop == "COUNT" and df == "" and gf != "":
            differs = True
            continue
        return False
    return differs


def score(got, gold, shape):
    """Return (verdict, attribution). Attribution names ONE cause."""
    if got is None:
        return "NO_PLAN", "not_generated"
    if sorted(got.get("group_fields") or []) != sorted(gold.get("group_fields") or []):
        return "WRONG", "group_field"
    equivalent = False
    if norm_measures(got) != norm_measures(gold):
        if count_field_only_diff(got, gold):
            equivalent = True
        else:
            g_ops = sorted(o for o, _ in norm_measures(got))
            d_ops = sorted(o for o, _ in norm_measures(gold))
            return "WRONG", "aggregate" if g_ops != d_ops else "measure_field"
    if norm_filters(got) != norm_filters(gold):
        if len(got.get("filters") or []) < len(gold.get("filters") or []):
            return "WRONG", "missing_filter"
        gf = {f[0] for f in norm_filters(got)}
        df = {f[0] for f in norm_filters(gold)}
        return "WRONG", "filter_field" if gf != df else "filter_value"
    if sorted(got.get("project") or []) != sorted(gold.get("project") or []):
        return "WRONG", "projection"
    if shape == "rank":
        gs = [(s.get("target"), (s.get("direction") or "").upper()) for s in (got.get("sort") or [])]
        ds = [(s.get("target"), (s.get("direction") or "").upper()) for s in (gold.get("sort") or [])]
        if gs != ds or (got.get("limit") or 0) != (gold.get("limit") or 0):
            return "WRONG", "ordering"
    if equivalent:
        return "EQUIV", "count_field_vs_rows"
    return "CORRECT", None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("out_dir")
    ap.add_argument("--json", default="")
    args = ap.parse_args()

    gold_doc = json.load(open(GOLD, encoding="utf-8"))
    gold = {i["id"]: i for i in gold_doc["items"]}
    raw = os.path.join(args.out_dir, "baseline", "raw")
    if not os.path.isdir(raw):
        raw = os.path.join(args.out_dir, "raw")
    if not os.path.isdir(raw):
        sys.exit("no raw/ directory under %s" % args.out_dir)

    rows, verdicts, causes = [], collections.Counter(), collections.Counter()
    allowed_fields_violations, enum_unverifiable = 0, 0
    for qid, item in gold.items():
        path = os.path.join(raw, qid + ".json")
        if not os.path.exists(path):
            continue
        doc = json.load(open(path, encoding="utf-8"))
        audit = (doc.get("planner") or {}).get("semantic_operation_planner") or {}
        plan = audit.get("ir_shadow_plan")
        outcome = audit.get("ir_shadow_outcome") or "absent"
        path = audit.get("state") or ""
        issued = audit.get("ir_shadow_issued_fields") or []

        if outcome == "absent":
            # The generator was never ASKED: the shadow hook did not cover
            # the route that answered this question. Treating that as a
            # missing plan credits an inexpressible item with a correct
            # abstention it never earned -- a FALSE CORRECT, which is what
            # happened to CDR-08. Unmeasured questions leave the denominator.
            rows.append((qid, "UNMEASURED", "instrument_gap", outcome, path))
            verdicts["UNMEASURED"] += 1
            continue

        if item.get("inexpressible") or item.get("gold") == "ABSTAIN":
            # Producing NO plan is the right OUTCOME here: a plan silently
            # answers a different question than the one asked.
            cause = ("inexpressible_answered" if item.get("inexpressible")
                     else "absent_family_answered")
            if plan is not None:
                rows.append((qid, "WRONG", cause, outcome, path))
                verdicts["WRONG"] += 1
                causes[cause] += 1
                continue
            # But this schema gives the generator no abstain move -- it always
            # fills a plan -- so it never declines on purpose. When no plan
            # appeared because generation FAILED (truncated, malformed, 500,
            # no_family), the safe result is a coincidence, not a capability.
            # Counting it as correct flatters the score, which is the same
            # mistake CDR-08 already taught once. Reported, not credited.
            if outcome in ("generated", "absent"):
                rows.append((qid, "CORRECT", None, outcome, path))
                verdicts["CORRECT"] += 1
            else:
                rows.append((qid, "SAFE", "no_plan_by_" + outcome.split(":")[0], outcome, path))
                verdicts["SAFE"] += 1
            continue
        if not isinstance(item.get("gold"), dict):
            continue

        v, cause = score(plan, item["gold"], item.get("shape"))
        rows.append((qid, v, cause, outcome, path))
        verdicts[v] += 1
        if cause:
            causes[cause] += 1
        # The enum is the safety property: a field outside the layer must be
        # structurally impossible, so any occurrence is a finding in itself.
        # The safety property is membership in the enum the generator was
        # ACTUALLY offered -- not the shape of the id. Uncurated catalogue
        # fields are hashed (fld_b4d5a2...) and are legitimate members, so
        # judging by shape reports violations that did not happen.
        if plan:
            if issued:
                for fid in fields_used(plan):
                    if fid not in issued:
                        allowed_fields_violations += 1
            else:
                enum_unverifiable += 1

    scored = sum(v for k, v in verdicts.items()
                 if k not in ("UNMEASURED", "SAFE"))
    correct = verdicts["CORRECT"]
    print("GOLDEN-IR — generated plans scored against hand-written gold")
    print("  scored              : %d" % scored)
    if scored:
        print("  plan-exact correct  : %d/%d = %.0f%%" % (correct, scored, 100.0 * correct / scored))
        equiv = verdicts["EQUIV"]
        if equiv:
            print("  + execution-equivalent: %d/%d = %.0f%%   [COUNT(field) on a"
                  " field with no NULLs -- same answer]"
                  % (correct + equiv, scored, 100.0 * (correct + equiv) / scored))
    print("  no plan generated   : %d" % verdicts["NO_PLAN"])
    if verdicts["UNMEASURED"]:
        print("  UNMEASURED          : %d   [generator never asked -- excluded, not scored]"
              % verdicts["UNMEASURED"])
    if verdicts["SAFE"]:
        print("  safe-by-failure     : %d   [no plan, but because generation FAILED --"
              " right outcome, not earned]" % verdicts["SAFE"])
    print("  out-of-enum field ids: %d   [expected 0 -- the enum makes it impossible]"
          % allowed_fields_violations)
    if enum_unverifiable:
        print("  enum UNVERIFIABLE    : %d   [issued field list not recorded for these]"
              % enum_unverifiable)
    if causes:
        print("\n  failure attribution (one cause each):")
        for cause, n in causes.most_common():
            print("    %-24s %d" % (cause, n))
    print("\n  %-8s %-10s %-22s %-26s %s" % ("ID", "VERDICT", "CAUSE", "SHADOW", "ROUTE"))
    for qid, v, cause, outcome, path in sorted(rows):
        print("  %-8s %-10s %-22s %-26s %s" % (qid, v, cause or "", outcome, path))
    if args.json:
        json.dump({"rows": rows, "verdicts": dict(verdicts), "causes": dict(causes)},
                  open(args.json, "w", encoding="utf-8"), indent=1)
        print("\nwrote %s" % args.json)


if __name__ == "__main__":
    main()
