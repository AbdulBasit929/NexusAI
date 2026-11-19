#!/usr/bin/env python3
"""Score the three-arm acceptance of the Latin re-read (gates L1 to L5 of docs/data/ocr-latin-reread-preregistration-20261117.md) from the JSONL files written by accept_router_runner.py
(arms today, easyocr, reread; the sets ur_pages and mixed). Runs in the evaluation image (editdistance):

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro --entrypoint python nexusai-media-eval-easyocr:20261117 \
        /repo/tools/media_data/accept_reread_score.py --dir /data/results/ocr --prefix accept2 --json /data/results/ocr/accept2-scores.json
"""
import argparse
import json
import statistics
import sys

sys.path.insert(0, "/repo/tools/media_data")
import editdistance  # noqa: E402
import ocr_eval as oe  # noqa: E402

ARMS = ("today", "easyocr", "reread")


def load(path):
    return {r["id"]: r for r in (json.loads(line) for line in open(path, encoding="utf-8"))}


def cer(r):
    ref, hyp = oe.normalise(r["truth"]).replace(" ", ""), oe.normalise(r["hyp"]).replace(" ", "")
    return editdistance.eval(ref, hyp) / max(1, len(ref))


def block(records):
    hit = total = 0
    by = {}
    for r in records:
        for ident in r.get("identifiers") or []:
            ok = ident["value"] in r["hyp"]
            a = by.setdefault(ident["type"], [0, 0])
            a[1] += 1
            a[0] += ok
            hit += ok
            total += 1
    return {"n": len(records), "cer_mean": statistics.mean(cer(r) for r in records), "ids_hit": hit, "ids_total": total, "ids_rate": hit / total if total else None, "by_type": by,
            "seconds_median": statistics.median(r["seconds"] for r in records), "seconds_max": max(r["seconds"] for r in records),
            "empty_or_error": sum(1 for r in records if r.get("error") or not oe.normalise(r["hyp"]))}


def pct(x):
    return "n/a" if x is None else f"{100 * x:.1f}%"


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--dir", required=True)
    ap.add_argument("--prefix", default="accept2")
    ap.add_argument("--json", default="")
    args = ap.parse_args(argv)
    data = {arm: load(f"{args.dir}/{args.prefix}-{arm}.jsonl") for arm in ARMS}
    shared = sorted(set.intersection(*(set(d) for d in data.values())))
    print(f"images in all three arms: {len(shared)} ({', '.join(f'{a} {len(d)}' for a, d in data.items())})")
    out = {"sets": {}, "gates": {}}
    print("| set | arm | n | CER mean | identifiers | by type | empty/error | s median (max) |\n|---|---|---|---|---|---|---|---|")
    for name in ("ur_pages", "mixed"):
        keys = [k for k in shared if data["today"][k]["set"] == name]
        for arm in ARMS:
            b = block([data[arm][k] for k in keys])
            out["sets"].setdefault(name, {})[arm] = b
            print(f"| {name} | {arm} | {b['n']} | {pct(b['cer_mean'])} | {b['ids_hit']}/{b['ids_total']} = {pct(b['ids_rate'])} | " + ", ".join(f"{t} {v[0]}/{v[1]}" for t, v in sorted(b["by_type"].items()))
                  + f" | {b['empty_or_error']} | {b['seconds_median']:.1f} ({b['seconds_max']:.1f}) |")
    s, g = out["sets"], out["gates"]
    g["L1"] = {"met": s["ur_pages"]["reread"]["ids_rate"] >= s["ur_pages"]["today"]["ids_rate"] and s["ur_pages"]["reread"]["ids_rate"] - s["ur_pages"]["easyocr"]["ids_rate"] >= 0.10,
               "reread": s["ur_pages"]["reread"]["ids_rate"], "today": s["ur_pages"]["today"]["ids_rate"], "easyocr": s["ur_pages"]["easyocr"]["ids_rate"]}
    plates_r = s["mixed"]["reread"]["by_type"].get("plate", [0, 0])[0]
    plates_t = s["mixed"]["today"]["by_type"].get("plate", [0, 0])[0]
    g["L2"] = {"met": s["mixed"]["reread"]["ids_rate"] >= s["mixed"]["today"]["ids_rate"] and plates_r >= 0.9 * plates_t, "reread": s["mixed"]["reread"]["ids_rate"], "today": s["mixed"]["today"]["ids_rate"],
               "plates_reread": plates_r, "plates_today": plates_t}
    g["L3"] = {"met": all(s[n]["reread"]["cer_mean"] <= s[n]["easyocr"]["cer_mean"] + 0.01 for n in s) and s["mixed"]["today"]["cer_mean"] - s["mixed"]["reread"]["cer_mean"] >= 0.10
               and s["ur_pages"]["today"]["cer_mean"] - s["ur_pages"]["reread"]["cer_mean"] >= 0.25,
               "cer": {n: {a: s[n][a]["cer_mean"] for a in ARMS} for n in s}}
    extra = s["ur_pages"]["reread"]["seconds_median"] - s["ur_pages"]["easyocr"]["seconds_median"]
    g["L4"] = {"met": s["ur_pages"]["reread"]["seconds_median"] <= 30 and extra <= 3, "median_urdu_page_reread": s["ur_pages"]["reread"]["seconds_median"], "added_seconds": extra}
    g["L5"] = {"met": all(s[n]["reread"]["empty_or_error"] == 0 for n in s), "empty_or_error": {n: s[n]["reread"]["empty_or_error"] for n in s}}
    print()
    for name in sorted(g):
        print(name, json.dumps(g[name], ensure_ascii=False, default=str))
    if args.json:
        json.dump(out, open(args.json, "w", encoding="utf-8"), ensure_ascii=False, indent=1, default=str)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
