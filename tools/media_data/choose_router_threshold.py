#!/usr/bin/env python3
"""Choose the router threshold T_low from the per-image shares written by measure_router_share.py (development rows only).

    python tools/media_data/choose_router_threshold.py C:/tmp/nexusai-media-data/results/ocr/router-share-dev-20261117.json

Rule written before the measurement (plan section 4.1): at least 98% of English pages and of receipts below T_low (Latin route); at least 90% of mixed pages and at least 98% of Urdu pages at or above
T_low (EasyOCR). Images with no probe answer (no letters found) are counted as NOT sent to EasyOCR (the router falls back to Tesseract's script pass, which is a separate decision).
The chosen value is the middle of the interval of values that meet all four conditions; if there is none, the value with the largest smallest margin is shown and flagged as a compromise.
"""
import json
import statistics
import sys

GROUPS = {"english": ("en_pages", None), "receipts": ("cord", None), "urdu": ("ur_pages", None), "mixed": ("mixed", None)}
NEED = {"english": ("below", 0.98), "receipts": ("below", 0.98), "urdu": ("above", 0.98), "mixed": ("above", 0.90)}


def rate(shares, threshold, side):
    if not shares:
        return None
    if side == "below":
        return sum(1 for s in shares if s is None or s < threshold) / len(shares)  # no probe answer = not sent to EasyOCR
    return sum(1 for s in shares if s is not None and s >= threshold) / len(shares)


def main(path):
    records = json.load(open(path, encoding="utf-8"))
    by = {name: [r["share"] for r in records if r["set"] == key[0]] for name, key in GROUPS.items()}
    mixed_kinds = {}
    for r in records:
        if r["set"] == "mixed":
            mixed_kinds.setdefault(r["group"], []).append(r["share"])
    print("images:", {k: len(v) for k, v in by.items()}, "no probe answer:", {k: sum(1 for s in v if s is None) for k, v in by.items()})
    for name, shares in {**by, **{f"mixed/{k}": v for k, v in mixed_kinds.items()}}.items():
        known = sorted(s for s in shares if s is not None)
        if known:
            q = lambda p: known[min(len(known) - 1, int(p * len(known)))]
            print(f"  {name:18} n={len(shares):3}  min {known[0]:.2f}  p10 {q(0.10):.2f}  median {statistics.median(known):.2f}  p90 {q(0.90):.2f}  max {known[-1]:.2f}")
    grid = [i / 100 for i in range(1, 100)]
    table = []
    for t in grid:
        row = {name: rate(by[name], t, NEED[name][0]) for name in GROUPS}
        margins = [row[name] - NEED[name][1] for name in GROUPS if row[name] is not None]
        table.append((t, row, min(margins)))
    feasible = [t for t, row, margin in table if margin >= 0]
    print()
    print("threshold | English below | receipts below | Urdu at/above | mixed at/above | smallest margin")
    for t, row, margin in table:
        if round(t * 100) % 5 == 0:
            print(f"  {t:.2f}    | " + " | ".join("   n/a" if row[n] is None else f"{100 * row[n]:6.1f}%" for n in GROUPS) + f" | {100 * margin:+.1f}")
    print()
    if feasible:
        low, high = min(feasible), max(feasible)
        chosen = round((low + high) / 2, 2)
        print(f"MET: every value from {low:.2f} to {high:.2f} meets all four conditions; the middle value {chosen:.2f} is chosen.")
    else:
        best = max(table, key=lambda x: x[2])
        print(f"NOT MET: no value meets all four conditions. Best compromise {best[0]:.2f} (smallest margin {100 * best[2]:+.1f} points): " + ", ".join(f"{n} {100 * best[1][n]:.1f}%" for n in GROUPS if best[1][n] is not None))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1]))
