#!/usr/bin/env python3
"""The IMPACT INDEX: which questions of a saved bank can a change of the Ask engine touch, so that only those are asked again.

    # 1. the index of what the saved runs recorded, and the input of the replay
    python -X utf8 tools/ask_eval/impact_index.py --bank docs/qa/verified-bank-20261102.json \\
        --results C:/tmp/nexusai-g4/vb_final.jsonl C:/tmp/nexusai-g4/vb_recheck_9d.jsonl --emit-replay C:/tmp/nexusai-g4/replay_in.jsonl

    # 2. the saved queries replayed under the CURRENT checks, with no model and no database (one Go spec; needs the heavy lock, see tools/dev/README.md)
    $env:ASK_REPLAY_IN = 'C:/tmp/nexusai-g4/replay_in.jsonl'; $env:ASK_REPLAY_OUT = 'C:/tmp/nexusai-g4/replay_out.jsonl'
    go test ./api/forensic_records -run TestForensicRecordsSynthesis "-ginkgo.focus=Ask engine replay" -count=1 -vet=off

    # 3. the index, with what the change touches, and the questions to ask again
    python -X utf8 tools/ask_eval/impact_index.py --bank ... --results ... --replay C:/tmp/nexusai-g4/replay_out.jsonl --out docs/data/engine-round-9e-impact-index-20261103 [--max 12]

Why: a full bank is eight hours on the CPU laptop and a change is checked by the cheapest tier that can catch it (tools/dev/README.md). The saved result files hold, for every question, the views, the
queries, the attempts and the repairs of the run that answered it. Three kinds of evidence say that a change can reach a question:

  * the question was MISSED, or needed a repair of a kind the change touches (the targets);
  * the saved query, replayed through the CURRENT obligations, now draws an obligation it did not draw when it was accepted (a regression risk), or the current frame reads the question's direction
    from the place of its identifier (the frame changed), or the query is a one-row ranking (a tie can now surface), or it takes an extreme with MAX or MIN (the ORDER rule changed);
  * the question is a NEIGHBOUR: it has the same type as a touched question (the same family and the same way of asking).

The tool reads files only. It never calls the engine and never writes to a case.
"""
import argparse
import json
import os
import re
import sys

# the repairs a change of round 9e can touch, by the signature that names the change
REPAIR_SIGNATURES = {
    "direction": ("OBLIGATION:DIRECTION", "OBLIGATION:DIRECTION_ROLE"),
    "order": ("OBLIGATION:ORDER",),
    "event_kind": ("OBLIGATION:EVENT_TYPE", "VALIDATOR:UNKNOWN_COLUMN"),
}
SLOW_MS = 240_000


def load_jsonl(path):
    rows = []
    with open(path, encoding="utf-8") as handle:
        for line in handle:
            if line.strip():
                rows.append(json.loads(line))
    return rows


def latest_results(paths):
    """The newest record of each question: a later file replaces an earlier one (a re-ask replaces the run it re-asked)."""
    out = {}
    for path in paths:
        for record in load_jsonl(path):
            out[record["id"]] = dict(record, _file=os.path.basename(path))
    return out


def build_entries(bank, results):
    entries = []
    for q in bank["questions"]:
        r = results.get(q["id"]) or {}
        repairs = r.get("repairs") or []
        entries.append({
            "id": q["id"], "area": q.get("area"), "type": q.get("type"), "question": q["question"], "expect_kind": (q.get("expect") or {}).get("kind"),
            "state": r.get("state") or "not run", "attempts": r.get("attempts"), "repairs": repairs, "views": r.get("views") or [], "sql": r.get("sql") or "",
            "seconds": round((r.get("total_ms") or 0) / 1000, 1), "error": r.get("error"), "result_file": r.get("_file"),
        })
    return entries


def emit_replay(entries, path):
    with open(path, "w", encoding="utf-8", newline="\n") as handle:
        for e in entries:
            handle.write(json.dumps({"id": e["id"], "question": e["question"], "sql": e["sql"], "views": e["views"]}, ensure_ascii=False) + "\n")
    print("%d queries -> %s" % (len(entries), path))


def classify(entries, replay):
    """Adds `touched_by` (the reasons a change can reach the question) and `role` (target, risk, neighbour or untouched) to every entry."""
    for e in entries:
        reasons = []
        outcome_good = e["state"] == "answered" and not e["repairs"]
        if e["state"] in ("declined", "abstained") or e.get("error"):
            reasons.append("missed:" + (e.get("error") or e["state"]))
        for name, kinds in REPAIR_SIGNATURES.items():
            if any(r in kinds for r in e["repairs"]):
                reasons.append("repair:" + name)
        if e["seconds"] * 1000 >= SLOW_MS or (e["attempts"] or 0) >= 3:
            reasons.append("slow-or-three-attempts")
        r = replay.get(e["id"])
        if r:
            if r.get("direction_role"):
                reasons.append("direction-from-place:" + r["direction_role"])
            if r.get("obligations") and e["state"] == "answered":
                reasons.append("saved-query-now-draws:" + ",".join(r["obligations"]))
            if r.get("tie_eligible"):
                reasons.append("one-row-ranking")
            if r.get("order_waived"):
                reasons.append("extreme-by-max-min")
            if r.get("validator"):
                reasons.append("saved-query-no-longer-validates:" + r["validator"])
        # the answer of a question that names a value of a column the query filters on now says which reading was used (a note, not a number)
        if "v_evidence_inventory" in e["views"] and re.search(r"\b(modality|detected_type)\s*=", e["sql"]):
            reasons.append("answer-note:value-of-a-column")
        e["touched_by"] = reasons
        e["role"] = "target" if any(x.startswith(("missed", "repair")) for x in reasons) else ("risk" if reasons else "untouched")
        e["clean_before"] = outcome_good
    touched_types = {(e["area"], e["type"]) for e in entries if e["role"] in ("target", "risk")}
    for e in entries:
        if e["role"] == "untouched" and (e["area"], e["type"]) in touched_types:
            e["role"] = "neighbour"
    return entries


def choose(entries, limit):
    """The questions to ask again. Every MISS is asked again (never cut). Then, in the order target, risk, neighbour, ONE question for each type and each kind of evidence (a type that has a miss and also questions that
    were answered after a repair of the touched kind gets one of each), until `limit` questions are chosen in all."""
    order = {"target": 0, "risk": 1, "neighbour": 2}
    pool = sorted((e for e in entries if e["role"] in order), key=lambda e: (order[e["role"]], e["id"]))
    chosen, seen = [], set()
    for e in pool:
        if any(x.startswith("missed") for x in e["touched_by"]):
            chosen.append(e)
            seen.add((e["area"], e["type"], "missed"))
    for e in pool:
        if e in chosen or len(chosen) >= limit:
            continue
        key = (e["area"], e["type"], "missed" if any(x.startswith("missed") for x in e["touched_by"]) else e["role"])
        if key in seen:
            continue
        seen.add(key)
        chosen.append(e)
    return chosen


def write_outputs(entries, chosen, out):
    counts = {}
    for e in entries:
        counts[e["role"]] = counts.get(e["role"], 0) + 1
    slim = [{k: v for k, v in e.items() if k != "sql"} | {"sql_chars": len(e["sql"])} for e in entries]     # the queries stay in the result files they were read from
    with open(out + ".json", "w", encoding="utf-8", newline="\n") as handle:
        json.dump({"questions": len(entries), "roles": counts, "reask": [e["id"] for e in chosen], "entries": slim}, handle, indent=1, ensure_ascii=False)
        handle.write("\n")
    lines = ["# Impact index", "", "%d questions in the saved bank; roles: %s." % (len(entries), ", ".join("%s %d" % (k, v) for k, v in sorted(counts.items()))), "",
             "| Question | Type | Saved outcome | Touched by |", "|---|---|---|---|"]
    for e in chosen:
        lines.append("| %s | %s/%s | %s (%s attempts, %ss) | %s |" % (e["id"], e["area"], e["type"], e["state"], e["attempts"], e["seconds"], "; ".join(e["touched_by"]) or "neighbour of " + e["type"]))
    with open(out + ".md", "w", encoding="utf-8", newline="\n") as handle:
        handle.write("\n".join(lines) + "\n")
    print("roles:", counts)
    print("ask again (%d): %s" % (len(chosen), ",".join(e["id"] for e in chosen)))


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--bank", required=True)
    ap.add_argument("--results", nargs="+", required=True, help="result files, oldest first; a later file replaces an earlier record of the same question")
    ap.add_argument("--replay", help="the output of the Go replay spec (obligations of the saved queries under the current checks)")
    ap.add_argument("--emit-replay", help="write the input of the Go replay spec here and stop")
    ap.add_argument("--out", help="write <out>.json and <out>.md")
    ap.add_argument("--max", type=int, default=12, help="how many questions to ask again, besides every target")
    args = ap.parse_args(argv)
    with open(args.bank, encoding="utf-8") as handle:
        bank = json.load(handle)
    entries = build_entries(bank, latest_results(args.results))
    if args.emit_replay:
        emit_replay(entries, args.emit_replay)
        return 0
    replay = {r["id"]: r for r in load_jsonl(args.replay)} if args.replay else {}
    classify(entries, replay)
    chosen = choose(entries, args.max)
    if args.out:
        write_outputs(entries, chosen, args.out)
    else:
        print("ask again (%d): %s" % (len(chosen), ",".join(e["id"] for e in chosen)))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
