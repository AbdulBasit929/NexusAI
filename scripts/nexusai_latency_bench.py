"""Latency benchmark for the forensic query API.

Single samples in this environment are not trustworthy to within an order of
magnitude: the same question measured 16.8 s once and 150/91/74 s on three
consecutive runs an hour later. This harness exists so a latency claim is
backed by a distribution and the machine state it was taken under, rather than
by whichever number was observed first.

It measures, it never tunes:

  * a warm-up run per question is executed and DISCARDED, so a cold model or a
    cold embedding catalogue is never reported as the steady state;
  * every question is repeated N times and reported as min/median/max, never as
    a single number;
  * host and container load are sampled around every run, because a fast
    reading taken on an idle machine and a slow one taken under load are not
    the same measurement;
  * the server-side breakdown (ir_generation_ms, decision_latency_ms, db, llm)
    is recorded alongside the wall time, and the UNACCOUNTED remainder is
    reported explicitly. An unaccounted majority means the instrument is still
    incomplete and no tuning conclusion may be drawn from the run.

Standard library only, matching nexusai_live_eval.py.

Usage:
  python scripts/nexusai_latency_bench.py \
      --golden evaluation/golden_questions_v1.json \
      --only CDR-01,ANPR-04,TWR-01 --repeat 3 \
      --out reports/latency-bench-20260924
"""
import argparse
import io
import json
import os
import statistics
import subprocess
import sys
import time
import urllib.request


def host_load():
    """Container CPU/memory at this instant. Best effort: a benchmark must not
    fail because a diagnostic did."""
    try:
        out = subprocess.run(
            ["docker", "stats", "--no-stream", "--format", "{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}"],
            capture_output=True, text=True, timeout=60).stdout
    except Exception:
        return {}
    load = {}
    for line in out.splitlines():
        parts = line.split("\t")
        if len(parts) >= 3 and parts[0].strip():
            load[parts[0].strip()] = {"cpu": parts[1].strip(), "mem": parts[2].strip()}
    return load


def ask(base, key, tenant, collection, question, timeout):
    body = json.dumps({"tenant_id": tenant, "collection_id": collection,
                       "query": question}).encode()
    request = urllib.request.Request(base + "/query/hybrid", data=body, method="POST", headers={
        "Content-Type": "application/json", "Authorization": "Bearer " + key,
        "X-Forensic-Tenant-ID": tenant, "X-Forensic-Actor-ID": "latency-bench",
        "X-Forensic-Subject-ID": "latency-bench", "X-Forensic-Actor-Role": "admin",
        "X-Forensic-Collection-ID": collection})
    started = time.time()
    with urllib.request.urlopen(request, timeout=timeout) as response:
        payload = json.loads(response.read().decode())
    return payload, int((time.time() - started) * 1000)


def breakdown(payload, wall_ms):
    telemetry = payload.get("telemetry") or {}
    planner = payload.get("planner") or {}
    audit = planner.get("semantic_operation_planner") or {}
    hybrid = {}
    for value in planner.values():
        if isinstance(value, dict) and "decision_latency_ms" in value:
            hybrid = value
            break
    parts = {
        "total_ms": telemetry.get("total_latency_ms") or wall_ms,
        "ir_generation_ms": audit.get("ir_generation_ms") or 0,
        "decision_ms": hybrid.get("decision_latency_ms") or 0,
        "db_ms": telemetry.get("db_latency_ms") or 0,
        "llm_ms": telemetry.get("llm_latency_ms") or 0,
    }
    accounted = parts["ir_generation_ms"] + parts["decision_ms"] + parts["db_ms"] + parts["llm_ms"]
    parts["accounted_ms"] = accounted
    parts["unaccounted_ms"] = max(0, parts["total_ms"] - accounted)
    parts["intent"] = payload.get("intent")
    parts["route"] = "+".join(payload.get("route") or [])
    return parts


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--golden", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--only", default="", help="comma-separated ids; default is every question")
    ap.add_argument("--repeat", type=int, default=3, help="timed runs per question")
    ap.add_argument("--warmup", type=int, default=1, help="discarded runs per question")
    ap.add_argument("--base", default=os.environ.get("NX_BASE", "http://localhost:8091"))
    ap.add_argument("--timeout", type=int, default=900)
    args = ap.parse_args()

    key = os.environ.get("FORENSIC_RECORDS_API_KEY")
    if not key:
        print("FORENSIC_RECORDS_API_KEY is required", file=sys.stderr)
        return 2
    tenant = os.environ.get("FORENSIC_RECORDS_TENANT_ID", "default")

    golden = json.load(io.open(args.golden, encoding="utf-8"))
    collections = golden["collections"]
    wanted = [q.strip() for q in args.only.split(",") if q.strip()]
    questions = [q for q in golden["questions"] if not wanted or q["id"] in wanted]

    os.makedirs(args.out, exist_ok=True)
    started_load = host_load()
    results = []

    for item in questions:
        qid, text = item["id"], item["q"]
        collection = collections[item["c"]]
        for _ in range(max(0, args.warmup)):
            try:
                ask(args.base, key, tenant, collection, text, args.timeout)
            except Exception as exc:
                print("%-9s warm-up failed: %s" % (qid, exc), flush=True)
        samples = []
        for run in range(args.repeat):
            before = host_load()
            try:
                payload, wall = ask(args.base, key, tenant, collection, text, args.timeout)
            except Exception as exc:
                print("%-9s run %d FAILED: %s" % (qid, run + 1, exc), flush=True)
                continue
            sample = breakdown(payload, wall)
            sample["wall_ms"] = wall
            sample["load_before"] = before
            samples.append(sample)
            print("%-9s run %d  wall=%6.1fs  ir_gen=%6.1fs  decision=%5.1fs  db=%4.1fs  unaccounted=%6.1fs" % (
                qid, run + 1, wall / 1000.0, sample["ir_generation_ms"] / 1000.0,
                sample["decision_ms"] / 1000.0, sample["db_ms"] / 1000.0,
                sample["unaccounted_ms"] / 1000.0), flush=True)
        if not samples:
            continue
        walls = sorted(s["wall_ms"] for s in samples)
        results.append({
            "id": qid, "question": text, "runs": len(samples),
            "wall_min_ms": walls[0], "wall_median_ms": int(statistics.median(walls)),
            "wall_max_ms": walls[-1],
            "spread_ratio": round(walls[-1] / walls[0], 2) if walls[0] else None,
            "ir_generation_median_ms": int(statistics.median([s["ir_generation_ms"] for s in samples])),
            "unaccounted_median_ms": int(statistics.median([s["unaccounted_ms"] for s in samples])),
            "intent": samples[0]["intent"], "route": samples[0]["route"],
            "samples": samples,
        })

    report = {
        "contract_version": "forensics.latency-benchmark/v1",
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%S"),
        "base": args.base, "repeat": args.repeat, "warmup": args.warmup,
        "load_at_start": started_load, "load_at_end": host_load(),
        "results": results,
    }
    path = os.path.join(args.out, "benchmark.json")
    json.dump(report, io.open(path, "w", encoding="utf-8"), indent=1, ensure_ascii=False)

    print("\n%-9s %-8s %-8s %-8s %-7s %-10s %s" % (
        "QID", "MIN", "MEDIAN", "MAX", "SPREAD", "IR_GEN(med)", "UNACCOUNTED(med)"))
    for row in results:
        print("%-9s %-8s %-8s %-8s %-7s %-10s %s" % (
            row["id"],
            "%.1fs" % (row["wall_min_ms"] / 1000.0),
            "%.1fs" % (row["wall_median_ms"] / 1000.0),
            "%.1fs" % (row["wall_max_ms"] / 1000.0),
            "%sx" % row["spread_ratio"],
            "%.1fs" % (row["ir_generation_median_ms"] / 1000.0),
            "%.1fs" % (row["unaccounted_median_ms"] / 1000.0)))
    medians = [r["wall_median_ms"] for r in results]
    if medians:
        ordered = sorted(medians)
        p95 = ordered[min(len(ordered) - 1, int(round(0.95 * (len(ordered) - 1))))]
        print("\nmedian of medians %.1fs · p95 of medians %.1fs · P3 gate is p95 < 15s" % (
            statistics.median(ordered) / 1000.0, p95 / 1000.0))
    print("written %s" % path)
    return 0


if __name__ == "__main__":
    sys.exit(main())
