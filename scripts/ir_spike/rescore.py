#!/usr/bin/env python3
"""Re-score saved WI-0 raw results with a corrected filter comparison.

WHY THIS EXISTS
The first scorer compared a filter's `value` field for every operator. That is
wrong for BETWEEN and IN: the executor reads ONLY `Values` for those two
(source_native_algebra.go:764-770 -- `filter.Value` is never referenced in the
IN/BETWEEN branches), and validation does not reject a populated `value`
alongside them. CDR-14 produced a BETWEEN filter with the correct bounds plus a
redundant `value`, which is execution-identical to gold and was scored WRONG.

Symmetrically, `values` is ignored for every other operator, so it must not be
compared there either.

This re-scores the STORED plans. No new inference is performed, so the model's
behaviour is untouched -- only the measurement of it is corrected. Both arms are
re-scored by the same code so they stay comparable.
"""
import json
import os
import statistics
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import ir_spike as S                                            # noqa: E402

SET_OPS = ("BETWEEN", "IN")
NULL_OPS = ("IS_NULL", "IS_NOT_NULL")


def norm_filters(plan):
    out = []
    for f in plan.get("filters") or []:
        op = (f.get("op") or "").upper()
        field_id = f.get("field_id") or ""
        if op in NULL_OPS:
            out.append((field_id, op, "", ()))
        elif op == "BETWEEN":
            # Ordered: values[0] is the lower bound, values[1] the upper.
            out.append((field_id, op, "",
                        tuple(v.strip().lower() for v in (f.get("values") or []))))
        elif op == "IN":
            out.append((field_id, op, "",
                        tuple(sorted(v.strip().lower() for v in (f.get("values") or [])))))
        else:
            out.append((field_id, op, (f.get("value") or "").strip().lower(), ()))
    return sorted(out)


def equivalent(got, gold, shape):
    if got is None or gold is None:
        return False
    if sorted(got.get("project") or []) != sorted(gold.get("project") or []):
        return False
    if sorted(got.get("group_fields") or []) != sorted(gold.get("group_fields") or []):
        return False

    def measures(plan):
        return sorted(((m.get("op") or "").upper(), m.get("field_id") or "")
                      for m in (plan.get("measures") or []))
    if measures(got) != measures(gold):
        return False
    if norm_filters(got) != norm_filters(gold):
        return False
    if shape == "rank":
        def sort_of(plan):
            return [(s.get("target"), (s.get("direction") or "").upper())
                    for s in (plan.get("sort") or [])]
        if sort_of(got) != sort_of(gold) or (got.get("limit") or 0) != (gold.get("limit") or 0):
            return False
    if shape == "rows" and (got.get("limit") or 0) != (gold.get("limit") or 0):
        return False
    return True


def main():
    with open(os.path.join(HERE, "gold_plans.json"), encoding="utf-8") as handle:
        gold_items = {i["id"]: i for i in json.load(handle)["items"]}

    for arm in sys.argv[1:]:
        path = arm if os.path.exists(arm) else None
        if not path:
            print("missing: %s" % arm)
            continue
        with open(path, encoding="utf-8") as handle:
            doc = json.load(handle)
        results = doc["results"]
        changed = []
        for r in results:
            if r.get("gold_kind") != "plan":
                continue
            item = gold_items[r["id"]]
            gold = item.get("gold")
            plan = r.get("plan")
            was = r.get("correct")
            if r.get("abstained") or plan is None:
                correct, cw = False, False
            else:
                correct = equivalent(plan, gold, item.get("shape"))
                cw = not correct
            if correct != was:
                changed.append((r["id"], was, correct))
            r["correct"], r["confident_wrong"] = correct, cw
            if correct:
                r["attribution"] = None
            elif r.get("abstained"):
                r["attribution"] = "abstained"
            else:
                r["attribution"] = S.attribute(plan, gold, item.get("shape"))

        scored = [r for r in results if r["gold_kind"] == "plan"]
        n = len(scored)
        correct = sum(1 for r in scored if r["correct"])
        cw = sum(1 for r in scored if r["confident_wrong"])
        ab = sum(1 for r in scored if r.get("abstained"))
        exact = sum(1 for r in scored if r.get("exact"))
        mal = sum(1 for r in results if r.get("malformed"))
        lat = [r["latency"] for r in results if r.get("latency")]
        inexpr = [r for r in results if r["gold_kind"] == "inexpressible"]
        absitems = [r for r in results if r["gold_kind"] == "abstain"]

        print("=" * 66)
        print("ARM %s (re-scored)" % doc["arm"])
        print("=" * 66)
        if changed:
            print("Verdict changes from the corrected filter comparison:")
            for qid, before, after in changed:
                print("  %-8s %s -> %s" % (qid, before, after))
        else:
            print("No verdict changed.")
        print("Expressible scored          : %d" % n)
        print("Execution-equivalent        : %d/%d = %.1f%%" % (correct, n, 100.0 * correct / n))
        print("Exact-IR                    : %d/%d = %.1f%%" % (exact, n, 100.0 * exact / n))
        print("Abstention (expressible)    : %d/%d = %.1f%%" % (ab, n, 100.0 * ab / n))
        print("CONFIDENT-WRONG             : %d/%d = %.1f%%" % (cw, n, 100.0 * cw / n))
        print("Malformed                   : %d" % mal)
        if lat:
            o = sorted(lat)
            p95 = o[min(len(o) - 1, int(round(0.95 * (len(o) - 1))))]
            print("Latency p50/p95/max         : %.1fs / %.1fs / %.1fs"
                  % (statistics.median(o), p95, o[-1]))
        print("Inexpressible abstained ok  : %d/%d" % (sum(1 for r in inexpr if r.get("correct")), len(inexpr)))
        print("Family-absent abstained ok  : %d/%d" % (sum(1 for r in absitems if r.get("correct")), len(absitems)))
        buckets = {}
        for r in scored:
            if r.get("attribution"):
                buckets[r["attribution"]] = buckets.get(r["attribution"], 0) + 1
        if buckets:
            print("Failure attribution:")
            for key, value in sorted(buckets.items(), key=lambda kv: -kv[1]):
                print("  %-18s %d" % (key, value))
        rate = 100.0 * correct / n
        verdict = "PROCEED" if rate >= 80 else ("PROCEED, REINFORCED" if rate >= 60 else "STOP - ESCALATE")
        print("Rule -> %s (%.1f%%)" % (verdict, rate))
        if mal > 0:
            print("OVERRIDE: malformed > 0 -> STOP AND ESCALATE")
        if 100.0 * cw / n > 10:
            print("OVERRIDE: confident-wrong > 10% -> STOP AND ESCALATE")
        print()
        with open(path.replace(".json", "_rescored.json"), "w", encoding="utf-8") as handle:
            json.dump(doc, handle, indent=2)


if __name__ == "__main__":
    main()
