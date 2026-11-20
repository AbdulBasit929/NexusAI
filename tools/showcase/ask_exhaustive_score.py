#!/usr/bin/env python3
"""Scores a run of tools/showcase/ask_exhaustive_bank.py against the gates of docs/roadmap/ask-exhaustive-search-plan-20261119.md.

    python -X utf8 tools/showcase/ask_exhaustive_score.py RUN.jsonl [--baseline BASE.jsonl] [--list]

Gates for a new-wordings bank (bank-B): full recall on at least 90% of the questions, mean recall at least 0.95, mean precision at least 0.9, no question that cites nothing where a true file
exists, median seconds under 15. Prints the numbers and, with --list, one line per question (misses first).
"""
import argparse
import json
import statistics
import sys


def load(path):
    return [json.loads(line) for line in open(path, encoding="utf-8") if line.strip()]


def summarise(rows):
    n = len(rows)
    full = sum(1 for r in rows if r["recall"] >= 0.999)
    nothing = sum(1 for r in rows if not r["cited"])
    precisions = [r["precision"] for r in rows if r["precision"] is not None]
    seconds = [r["seconds"] for r in rows if r.get("seconds") is not None]
    return {
        "questions": n, "full_recall": full, "full_recall_pct": 100 * full / n if n else 0, "mean_recall": sum(r["recall"] for r in rows) / n if n else 0,
        "mean_precision": sum(precisions) / len(precisions) if precisions else 0, "cites_nothing": nothing, "median_seconds": statistics.median(seconds) if seconds else 0,
    }


def show(label, s):
    print(f"{label}: {s['questions']} questions | full recall {s['full_recall']} ({s['full_recall_pct']:.0f}%) | mean recall {s['mean_recall']:.2f} | mean precision {s['mean_precision']:.2f} | "
          f"cites nothing {s['cites_nothing']} | median {s['median_seconds']} s")


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("run")
    ap.add_argument("--baseline")
    ap.add_argument("--list", action="store_true")
    args = ap.parse_args(argv)
    rows = load(args.run)
    s = summarise(rows)
    show("run     ", s)
    if args.baseline:
        base = {r["n"]: r for r in load(args.baseline)}
        same = [base[r["n"]] for r in rows if r["n"] in base]
        show("baseline", summarise(same))
    gates = {
        "full recall >= 90%": s["full_recall_pct"] >= 90, "mean recall >= 0.95": s["mean_recall"] >= 0.95, "mean precision >= 0.9": s["mean_precision"] >= 0.9,
        "cites nothing = 0": s["cites_nothing"] == 0, "median < 15 s": s["median_seconds"] < 15,
    }
    for name, ok in gates.items():
        print(("  MET     " if ok else "  NOT MET ") + name)
    if args.list:
        for r in sorted(rows, key=lambda r: (r["recall"], r["n"])):
            precision = "n/a " if r["precision"] is None else f"{r['precision']:.2f}"
            missing = [f for f in r["truth"] if f not in r["cited"]]
            extra = [f for f in r["cited"] if f not in r["truth"]]
            print(f"{r['n']:3} {r['probe']:12} recall {r['recall']:.2f} prec {precision} {r['seconds']:>6} s | {r['question'][:60]}")
            if missing:
                print(f"      missing: {', '.join(missing)}")
            if extra:
                print(f"      extra:   {', '.join(extra)}")
            if missing or extra:
                print(f"      answer:  {r['answer'][:200]}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
