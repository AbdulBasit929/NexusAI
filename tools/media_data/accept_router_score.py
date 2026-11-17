#!/usr/bin/env python3
"""Score the product-level acceptance runs (gates R1 to R7 of docs/data/ocr-easyocr-acceptance-preregistration-20261117.md) from the two JSONL files written by accept_router_runner.py.

Runs in the evaluation image (editdistance); pure scoring, no model is loaded:

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro --entrypoint python nexusai-media-eval-easyocr:20261117 \
        /repo/tools/media_data/accept_router_score.py --today /data/results/ocr/accept-today.jsonl --easyocr /data/results/ocr/accept-easyocr.jsonl [--json out.json]
"""
import argparse
import json
import random
import statistics
import sys

sys.path.insert(0, "/repo/tools/media_data")
import editdistance  # noqa: E402
import ocr_eval as oe  # noqa: E402


def load(path):
    return {r["id"]: r for r in (json.loads(line) for line in open(path, encoding="utf-8"))}


def metrics(record):
    ref = oe.normalise(record["truth"]).replace(" ", "")
    hyp = oe.normalise(record["hyp"]).replace(" ", "")
    ids = record.get("identifiers") or []
    hit = sum(1 for i in ids if i["value"] in record["hyp"])
    return {"cer": (editdistance.eval(ref, hyp) / len(ref)) if ref else 0.0, "edits": editdistance.eval(ref, hyp), "ref_chars": len(ref), "f1": oe.bag_f1(record["truth"], record["hyp"]),
            "ids_total": len(ids), "ids_hit": hit, "empty": not hyp or bool(record.get("error")), "seconds": record["seconds"]}


def block(records):
    rows = [metrics(r) for r in records]
    n = len(rows)
    ids_total, ids_hit = sum(r["ids_total"] for r in rows), sum(r["ids_hit"] for r in rows)
    rng = random.Random(1)
    draws = sorted(sum(x["edits"] for x in s) / max(1, sum(x["ref_chars"] for x in s)) for s in (rng.choices(rows, k=n) for _ in range(1000))) if n > 1 else [0, 0]
    return {"n": n, "cer_mean": statistics.mean(r["cer"] for r in rows), "cer_pooled": sum(r["edits"] for r in rows) / max(1, sum(r["ref_chars"] for r in rows)),
            "cer_pooled_ci": [draws[25], draws[974 if n > 1 else 1]], "f1": statistics.mean(r["f1"] for r in rows), "ids_hit": ids_hit, "ids_total": ids_total,
            "ids_rate": (ids_hit / ids_total) if ids_total else None, "empty_share": sum(1 for r in rows if r["empty"]) / n, "seconds_median": statistics.median(r["seconds"] for r in rows),
            "seconds_max": max(r["seconds"] for r in rows)}


def pct(x):
    return "n/a" if x is None else f"{100 * x:.1f}%"


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--today", required=True)
    ap.add_argument("--easyocr", required=True)
    ap.add_argument("--json", default="")
    args = ap.parse_args(argv)
    today, easy = load(args.today), load(args.easyocr)
    shared = sorted(set(today) & set(easy))
    print(f"images in both arms: {len(shared)} (today {len(today)}, easyocr {len(easy)})")
    out = {"sets": {}, "gates": {}}
    sets = sorted({today[k]["set"] for k in shared})
    print("| set | arm | n | CER mean | CER pooled [95%] | F1 | identifiers | empty/error | s median (max) | routed to EasyOCR |")
    print("|---|---|---|---|---|---|---|---|---|---|")
    for name in sets:
        keys = [k for k in shared if today[k]["set"] == name]
        for arm, data in (("today", today), ("easyocr", easy)):
            recs = [data[k] for k in keys]
            b = block(recs)
            routed = sum(1 for r in recs if r["engine"] == "easyocr-ur+en") / len(recs)
            b["routed_to_easyocr"] = routed
            out["sets"].setdefault(name, {})[arm] = b
            ids = f"{b['ids_hit']}/{b['ids_total']} = {pct(b['ids_rate'])}" if b["ids_total"] else "-"
            print(f"| {name} | {arm} | {b['n']} | {pct(b['cer_mean'])} | {pct(b['cer_pooled'])} [{pct(b['cer_pooled_ci'][0])}, {pct(b['cer_pooled_ci'][1])}] | {pct(b['f1'])} | {ids} | {pct(b['empty_share'])} | {b['seconds_median']:.1f} ({b['seconds_max']:.1f}) | {pct(routed)} |")
    g = out["gates"]
    s = out["sets"]

    def arm(name, which):
        return (s.get(name) or {}).get(which)

    if arm("unb", "easyocr"):
        e, t = arm("unb", "easyocr"), arm("unb", "today")
        g["R1"] = {"met": e["cer_pooled"] <= 0.25 and t["cer_pooled"] - e["cer_pooled"] >= 0.10, "easyocr": e["cer_pooled"], "today": t["cer_pooled"], "n": e["n"]}
    # Naskh and Nastaliq lines are told apart by the group string the runner wrote (kind-style-variant)
    lines_e = [r for k in shared for r in [easy[k]] if r["set"] == "ur_lines"]
    lines_t = [r for k in shared for r in [today[k]] if r["set"] == "ur_lines"]
    if lines_e:
        verdicts = {}
        for style, limit in (("naskh", 0.15), ("nastaliq", 0.25)):
            e_part = [r for r in lines_e if f"-{style}-" in r["group"]]
            t_part = [r for r in lines_t if f"-{style}-" in r["group"]]
            if e_part:
                e_cer, t_cer = block(e_part)["cer_mean"], block(t_part)["cer_mean"]
                verdicts[style] = {"easyocr": e_cer, "today": t_cer, "met": e_cer <= limit and e_cer <= t_cer, "n": len(e_part)}
        pages_e, pages_t = arm("ur_pages", "easyocr"), arm("ur_pages", "today")
        if pages_e:
            verdicts["pages"] = {"easyocr": pages_e["cer_mean"], "today": pages_t["cer_mean"], "met": pages_e["cer_mean"] <= 0.15 and pages_e["cer_mean"] <= pages_t["cer_mean"], "n": pages_e["n"]}
        g["R2"] = {"met": all(v["met"] for v in verdicts.values()), "parts": verdicts}
    if arm("mixed", "easyocr"):
        e, t = arm("mixed", "easyocr"), arm("mixed", "today")
        g["R3"] = {"met": (e["ids_rate"] or 0) >= 0.90 and (e["ids_rate"] or 0) >= (t["ids_rate"] or 0) and t["cer_mean"] - e["cer_mean"] >= 0.10,
                   "ids_easyocr": e["ids_rate"], "ids_today": t["ids_rate"], "cer_easyocr": e["cer_mean"], "cer_today": t["cer_mean"], "f1_easyocr_reported_not_gated": e["f1"], "f1_today": t["f1"]}
    if arm("en", "easyocr"):
        e, t = arm("en", "easyocr"), arm("en", "today")
        g["R4"] = {"met": (e["ids_rate"] or 0) >= (t["ids_rate"] or 0) - 0.02 and e["routed_to_easyocr"] <= 0.02, "ids_easyocr": e["ids_rate"], "ids_today": t["ids_rate"], "english_sent_to_easyocr": e["routed_to_easyocr"]}
    routing = {name: (arm(name, "easyocr") or {}).get("routed_to_easyocr") for name in ("en", "ur_pages", "ur_lines", "mixed", "unb")}
    mixed_kinds = {}
    for k in shared:
        r = easy[k]
        if r["set"] == "mixed":
            mixed_kinds.setdefault(r["group"].split("-")[0], []).append(1 if r["engine"] == "easyocr-ur+en" else 0)
    # amendment A1 of the acceptance plan: English at most 2% and Urdu pages at least 98% to EasyOCR; the mixed images are reported per kind and not gated
    g["R5"] = {"met": routing["ur_pages"] is not None and routing["en"] is not None and routing["ur_pages"] >= 0.98 and routing["en"] <= 0.02, "routed_to_easyocr": routing,
               "mixed_by_kind_reported_not_gated": {k: sum(v) / len(v) for k, v in sorted(mixed_kinds.items())}}
    pages = arm("ur_pages", "easyocr")
    if pages:
        g["R6"] = {"seconds_median_urdu_page": pages["seconds_median"], "met_time": pages["seconds_median"] <= 30, "memory": "measured separately (speed run)"}
    empties = {name: (arm(name, "easyocr") or {}).get("empty_share") for name in sets}
    g["R7"] = {"met": all(v is not None and v <= 0.05 for v in empties.values()), "empty_or_error_share": empties}
    print()
    for name in sorted(g):
        print(name, json.dumps(g[name], ensure_ascii=False, default=str))
    if args.json:
        json.dump(out, open(args.json, "w", encoding="utf-8"), ensure_ascii=False, indent=1, default=str)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
