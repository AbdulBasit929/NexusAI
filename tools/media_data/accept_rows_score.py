#!/usr/bin/env python3
"""Score the acceptance of the table-row rule (gates T1 to T3 of docs/data/ocr-table-row-order-preregistration-20261119.md). Runs in the evaluation image (editdistance):

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro --entrypoint python nexusai-media-eval-easyocr:20261117 \
        /repo/tools/media_data/accept_rows_score.py --dir /data/results/ocr --prefix accept3 --json /data/results/ocr/accept3-scores.json

Files: <prefix>-reread.jsonl and <prefix>-rows.jsonl (routed ur_pages and mixed), <prefix>-stmt-reread.jsonl and <prefix>-stmt-rows.jsonl (statements read directly by EasyOCR).
"""
import argparse
import json
import statistics
import sys

sys.path.insert(0, "/repo/tools/media_data")
import editdistance  # noqa: E402
import ocr_eval as oe  # noqa: E402


def load(path):
    return {r["id"]: r for r in (json.loads(line) for line in open(path, encoding="utf-8"))}


def cer(r):
    ref, hyp = oe.normalise(r["truth"]).replace(" ", ""), oe.normalise(r["hyp"]).replace(" ", "")
    return editdistance.eval(ref, hyp) / max(1, len(ref))


def block(records):
    hit = sum(1 for r in records for i in r.get("identifiers") or [] if i["value"] in r["hyp"])
    total = sum(len(r.get("identifiers") or []) for r in records)
    return {"n": len(records), "cer_mean": statistics.mean(cer(r) for r in records), "ids_hit": hit, "ids_total": total, "ids_rate": hit / total if total else None,
            "seconds_median": statistics.median(r["seconds"] for r in records), "empty_or_error": sum(1 for r in records if r.get("error") or not oe.normalise(r["hyp"]))}


def pct(x):
    return "n/a" if x is None else f"{100 * x:.1f}%"


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--dir", required=True)
    ap.add_argument("--prefix", default="accept3")
    ap.add_argument("--json", default="")
    args = ap.parse_args(argv)
    out = {"routed": {}, "forced": {}, "gates": {}}
    print("| part | set | arm | n | CER mean | identifiers | empty/error | s median |\n|---|---|---|---|---|---|---|---|")
    arms = {}
    for part, names in (("routed", ("reread", "rows")), ("forced", ("stmt-reread", "stmt-rows"))):
        data = {a: load(f"{args.dir}/{args.prefix}-{a}.jsonl") for a in names}
        shared = sorted(set.intersection(*(set(d) for d in data.values())))
        sets = sorted({data[names[0]][k]["set"] for k in shared})
        for name in sets:
            keys = [k for k in shared if data[names[0]][k]["set"] == name]
            for arm in names:
                b = block([data[arm][k] for k in keys])
                out[part].setdefault(name, {})[arm] = b
                print(f"| {part} | {name} | {arm} | {b['n']} | {pct(b['cer_mean'])} | {b['ids_hit']}/{b['ids_total']} = {pct(b['ids_rate'])} | {b['empty_or_error']} | {b['seconds_median']:.1f} |")
    r, f, g = out["routed"], out["forced"], out["gates"]
    g["T1"] = {"met": all(r[n]["rows"]["cer_mean"] <= r[n]["reread"]["cer_mean"] + 0.005 and (r[n]["rows"]["ids_rate"] or 0) >= (r[n]["reread"]["ids_rate"] or 0) and r[n]["rows"]["empty_or_error"] == 0 for n in r),
               "cer": {n: {a: r[n][a]["cer_mean"] for a in ("reread", "rows")} for n in r}, "ids": {n: {a: r[n][a]["ids_rate"] for a in ("reread", "rows")} for n in r}}
    s = f["mixed"]
    g["T2"] = {"met": s["stmt-reread"]["cer_mean"] - s["stmt-rows"]["cer_mean"] >= 0.10 and (s["stmt-rows"]["ids_rate"] or 0) >= (s["stmt-reread"]["ids_rate"] or 0),
               "cer_reread": s["stmt-reread"]["cer_mean"], "cer_rows": s["stmt-rows"]["cer_mean"], "ids_reread": s["stmt-reread"]["ids_rate"], "ids_rows": s["stmt-rows"]["ids_rate"], "n": s["stmt-rows"]["n"]}
    g["T3"] = {"met": abs(r["ur_pages"]["rows"]["seconds_median"] - r["ur_pages"]["reread"]["seconds_median"]) <= 1.0, "median_seconds": {a: r["ur_pages"][a]["seconds_median"] for a in ("reread", "rows")}}
    print()
    for name in sorted(g):
        print(name, json.dumps(g[name], ensure_ascii=False, default=str))
    if args.json:
        json.dump(out, open(args.json, "w", encoding="utf-8"), ensure_ascii=False, indent=1, default=str)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
