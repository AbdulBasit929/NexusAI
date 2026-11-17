#!/usr/bin/env python3
"""Score the sealed runs of OCR bake-off v2 (docs/data/ocr-bakeoff-v2-preregistration-20261117.md) against the pre-registered gates U1 to U6.

Reads the result files written by ocr_eval.py (<data>/results/ocr/*-sealed-*.json) and by nastaliq_lab.py (<data>/results/nastaliq/{t,e}-sealed.jsonl). Pure Python, no third-party packages; it never
reads a sealed image or a reference text, only the per-row numbers the runs already wrote.

    python tools/media_data/score_ocr_bakeoff_v2.py [--data C:/tmp/nexusai-media-data] [--json out.json]

What the per-row files hold: character error rate per row (not edit counts), bag-of-words F1, identifiers hit and total, seconds, a failed flag. So the character error shown for the generated sets is the
**mean of per-row rates** (95% interval by bootstrap over rows); the newspaper set has edit counts and is **pooled** as the plan says. The word error rate is not stored, so the order gap of the plan
(F1 minus one minus WER) cannot be computed from these files; the character error is shown beside F1 instead, and the missing measure is said in the report.
"""
import argparse
import glob
import json
import os
import random
import statistics
import sys

DAY = "20261117"
SETS = {  # set -> {reader id: engine spec prefix as written in the file name}
    "mixed": {"T": "tess_best_urd-eng_3", "P": "onnx_latin_page", "E": "easyocr_ur-en"},
    "ur_lines_naskh": {"T": "tess_best_urd-eng_7", "E": "easyocr_ur-en"},
    "ur_lines_nastaliq": {"T": "tess_best_urd-eng_7", "E": "easyocr_ur-en"},
    "ur_pages": {"T": "tess_best_urd-eng_3", "E": "easyocr_ur-en"},
    "en_pages": {"P": "onnx_latin_page", "E": "easyocr_ur-en", "Een": "easyocr_en"},
    "cord": {"P": "onnx_latin_page", "Een": "easyocr_en"},
}


def load_rows(data, set_name, engine):
    pattern = os.path.join(data, "results", "ocr", f"{set_name}-sealed-{engine}-{DAY}-*.json")
    files = sorted(glob.glob(pattern))
    if not files:
        return None, []
    return files[-1], json.load(open(files[-1], encoding="utf-8"))["rows"]


def bootstrap(values, statistic, draws=2000, seed=1):
    rng = random.Random(seed)
    results = sorted(statistic([values[rng.randrange(len(values))] for _ in values]) for _ in range(draws))
    return results[int(0.025 * draws)], results[int(0.975 * draws) - 1]


def summarise(rows):
    n = len(rows)
    cers = [r["cer"] for r in rows]
    f1s = [r["f1"] for r in rows]
    ids_total, ids_hit = sum(r["ids_total"] for r in rows), sum(r["ids_hit"] for r in rows)
    by_type = {}
    for r in rows:
        for kind, pair in (r.get("ids_by_type") or {}).items():
            hit, total = (pair if isinstance(pair, (list, tuple)) else (pair.get("hit", 0), pair.get("total", 0)))
            acc = by_type.setdefault(kind, [0, 0])
            acc[0] += hit
            acc[1] += total
    bad = sum(1 for r in rows if r.get("failed") or r["cer"] >= 1.0)
    seconds = [r["seconds"] for r in rows]
    low, high = bootstrap(cers, statistics.mean) if n > 1 else (cers[0], cers[0])
    return {
        "rows": n, "cer_mean": statistics.mean(cers), "cer_median": statistics.median(cers), "cer_ci": [low, high], "f1_mean": statistics.mean(f1s),
        "ids_hit": ids_hit, "ids_total": ids_total, "ids_rate": (ids_hit / ids_total) if ids_total else None, "ids_by_type": {k: [v[0], v[1]] for k, v in sorted(by_type.items())},
        "empty_or_runaway": bad, "empty_or_runaway_share": bad / n, "seconds_median": statistics.median(seconds), "seconds_max": max(seconds),
    }


def newspaper(path):
    if not os.path.exists(path):
        return None
    records = [json.loads(line) for line in open(path, encoding="utf-8")]
    edits, chars = sum(r["edits"] for r in records), sum(r["ref_chars"] for r in records)
    low, high = bootstrap(records, lambda rs: sum(r["edits"] for r in rs) / sum(r["ref_chars"] for r in rs))
    return {"rows": len(records), "cer_pooled": edits / chars, "cer_ci": [low, high], "loops": sum(1 for r in records if r["loop"]),
            "empty": sum(1 for r in records if r["hyp_chars"] == 0), "seconds_median": statistics.median(r["seconds"] for r in records), "seconds_max": max(r["seconds"] for r in records)}


def pct(x):
    return "n/a" if x is None else f"{100 * x:.1f}%"


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--data", default="C:/tmp/nexusai-media-data")
    ap.add_argument("--json", default="")
    ap.add_argument("--breakdown", action="store_true", help="also print the mixed set by page kind and every generated set by variant, and the identifiers by type")
    args = ap.parse_args(argv)
    out = {"sets": {}, "gates": {}}
    for set_name, readers in SETS.items():
        out["sets"][set_name] = {}
        for reader, engine in readers.items():
            path, rows = load_rows(args.data, set_name, engine)
            if not rows:
                out["sets"][set_name][reader] = None
                continue
            out["sets"][set_name][reader] = {"file": os.path.basename(path), **summarise(rows)}
    out["sets"]["unb"] = {"T": newspaper(os.path.join(args.data, "results", "nastaliq", "t-sealed.jsonl")), "E": newspaper(os.path.join(args.data, "results", "nastaliq", "e-sealed.jsonl"))}

    print("| set | reader | rows | CER (mean per row, 95% CI) | median CER | F1 | identifiers | empty/runaway | s/row median (max) |")
    print("|---|---|---|---|---|---|---|---|---|")
    for set_name, readers in out["sets"].items():
        for reader, s in readers.items():
            if s is None:
                print(f"| {set_name} | {reader} | not run | | | | | | |")
            elif set_name == "unb":
                print(f"| {set_name} | {reader} | {s['rows']} | {pct(s['cer_pooled'])} pooled [{pct(s['cer_ci'][0])}, {pct(s['cer_ci'][1])}] | | | | loops {s['loops']}, empty {s['empty']} | {s['seconds_median']:.1f} ({s['seconds_max']:.1f}) |")
            else:
                ids = f"{s['ids_hit']}/{s['ids_total']} = {pct(s['ids_rate'])}" if s["ids_total"] else "-"
                print(f"| {set_name} | {reader} | {s['rows']} | {pct(s['cer_mean'])} [{pct(s['cer_ci'][0])}, {pct(s['cer_ci'][1])}] | {pct(s['cer_median'])} | {pct(s['f1_mean'])} | {ids} | {s['empty_or_runaway']} ({pct(s['empty_or_runaway_share'])}) | {s['seconds_median']:.1f} ({s['seconds_max']:.1f}) |")

    sets = out["sets"]
    gates = out["gates"]

    def get(set_name, reader):
        return (sets.get(set_name) or {}).get(reader)

    unb_e, unb_t = get("unb", "E"), get("unb", "T")
    if unb_e and unb_t:
        gates["U1"] = {"met": unb_e["cer_pooled"] <= 0.25 and (unb_t["cer_pooled"] - unb_e["cer_pooled"]) >= 0.10, "E": unb_e["cer_pooled"], "T": unb_t["cer_pooled"], "rows_E": unb_e["rows"], "rows_T": unb_t["rows"],
                      "complete": unb_e["rows"] == 60 and unb_t["rows"] == 60}
    naskh_e, naskh_t, nast_e, nast_t = get("ur_lines_naskh", "E"), get("ur_lines_naskh", "T"), get("ur_lines_nastaliq", "E"), get("ur_lines_nastaliq", "T")
    if naskh_e and naskh_t and nast_e and nast_t:
        gates["U2"] = {"met": naskh_e["cer_mean"] <= 0.15 and nast_e["cer_mean"] <= 0.25 and naskh_e["cer_mean"] <= naskh_t["cer_mean"] and nast_e["cer_mean"] <= nast_t["cer_mean"],
                       "naskh_E": naskh_e["cer_mean"], "naskh_T": naskh_t["cer_mean"], "nastaliq_E": nast_e["cer_mean"], "nastaliq_T": nast_t["cer_mean"]}
    mixed_e = get("mixed", "E")
    if mixed_e:
        gates["U3"] = {"met": (mixed_e["ids_rate"] or 0) >= 0.90 and mixed_e["f1_mean"] >= 0.85, "ids": mixed_e["ids_rate"], "f1": mixed_e["f1_mean"]}
    en_p, en_e, en_een = get("en_pages", "P"), get("en_pages", "E"), get("en_pages", "Een")
    if en_p and (en_e or en_een):
        best = max(x["ids_rate"] for x in (en_e, en_een) if x)
        gates["U4"] = {"met": best >= en_p["ids_rate"] - 0.02, "P": en_p["ids_rate"], "E": en_e and en_e["ids_rate"], "Een": en_een and en_een["ids_rate"]}
    pages = [x for x in (get("en_pages", "E"), get("ur_pages", "E")) if x]
    if pages:
        gates["U5"] = {"seconds_per_page_median": [x["seconds_median"] for x in pages], "seconds_per_page_max": [x["seconds_max"] for x in pages], "memory": "not recorded in the sealed runs (measured separately)",
                       "met_time": all(x["seconds_median"] <= 30 for x in pages)}
    shares = [x["empty_or_runaway_share"] for name in ("mixed", "ur_lines_naskh", "ur_lines_nastaliq", "ur_pages", "en_pages") for x in [get(name, "E")] if x]
    if shares:
        unb = unb_e and (unb_e["loops"] + unb_e["empty"]) / unb_e["rows"]
        gates["U6"] = {"met": max(shares + ([unb] if unb is not None else [])) <= 0.05, "shares": shares, "newspaper_loops_or_empty_share": unb}
    print()
    for name in ("U1", "U2", "U3", "U4", "U5", "U6"):
        print(name, json.dumps(gates.get(name), ensure_ascii=False, default=str))
    if args.breakdown:
        print()
        print("| set | split | reader | rows | CER mean | F1 | identifiers |")
        print("|---|---|---|---|---|---|---|")
        for set_name, readers in SETS.items():
            for reader, engine in readers.items():
                path, rows = load_rows(args.data, set_name, engine)
                if not rows:
                    continue
                splits = {}
                for r in rows:
                    splits.setdefault(("variant", r["group"]), []).append(r)
                    if set_name == "mixed":
                        splits.setdefault(("kind", r["id"].split("-")[1]), []).append(r)
                for (axis, value), part in sorted(splits.items()):
                    t = summarise(part)
                    ids = f"{t['ids_hit']}/{t['ids_total']} = {pct(t['ids_rate'])}" if t["ids_total"] else "-"
                    print(f"| {set_name} | {axis} {value} | {reader} | {t['rows']} | {pct(t['cer_mean'])} | {pct(t['f1_mean'])} | {ids} |")
        print()
        print("identifiers by type (hit/total), mixed set")
        for reader in SETS["mixed"]:
            s_ = (sets.get("mixed") or {}).get(reader)
            if s_:
                print(f"  {reader}: " + ", ".join(f"{k} {v[0]}/{v[1]}" for k, v in s_["ids_by_type"].items()))
    if args.json:
        json.dump(out, open(args.json, "w", encoding="utf-8"), ensure_ascii=False, indent=1, default=str)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
