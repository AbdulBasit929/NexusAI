#!/usr/bin/env python3
"""Live golden-question evaluation for the NexusAI forensic query API.

Runs every question in a golden set against the running forensic-records API,
grades the returned result values against SQL-derived oracle values stored in
the golden file (never against the API's own output), and separately grades
whether the analyst-facing answer text actually states the answer.

Standard library only. Reads FORENSIC_RECORDS_API_KEY / tenant from
.env.forensic-runtime.local (gitignored); never prints the key.

Usage:
  python scripts/nexusai_live_eval.py \
      --golden reports/nexusai-tl-audit-20260918/golden_questions_v0.json \
      --out reports/nexusai-tl-audit-20260918/baseline
"""
import argparse
import copy
import json
import os
import re
import sys
import time
import urllib.error
import urllib.request

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# Keys that echo the question or describe planning; excluded when searching
# for expected values so the question text can never satisfy its own check.
ECHO_KEYS = {
    "query_understanding", "investigation_context", "capability_snapshot",
    "execution_plan", "planner", "query_plan", "telemetry", "plan_validation",
    "policy", "capability", "resolved_capability", "coverage", "question",
    "original_question", "normalized_question", "current_question", "conversation_context",
}
ROW_NOISE_KEYS = {"metadata", "provenance", "source_rows", "citations", "row_hash",
                  "record_id", "batch_id", "evidence_id", "version_id", "file_id", "row_number"}
NON_ANSWER_STATES = {"no_results", "empty", "unsupported", "clarification_required",
                     "needs_clarification", "unavailable", "insufficient_evidence"}


def load_env():
    vals = {}
    path = os.path.join(REPO, ".env.forensic-runtime.local")
    with open(path, encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                vals[k] = v.strip().strip('"')
    return vals


def post(base, env, collection, question, timeout):
    body = json.dumps({"tenant_id": env.get("FORENSIC_RECORDS_TENANT_ID", "default"),
                       "collection_id": collection, "query": question}).encode()
    req = urllib.request.Request(base + "/query/hybrid", data=body, method="POST", headers={
        "Authorization": "Bearer " + env["FORENSIC_RECORDS_API_KEY"],
        "X-Forensic-Tenant-ID": env.get("FORENSIC_RECORDS_TENANT_ID", "default"),
        "X-Forensic-Actor-ID": "live-eval", "X-Forensic-Subject-ID": "live-eval",
        "X-Forensic-Actor-Role": "admin", "X-Forensic-Collection-ID": collection,
        "Content-Type": "application/json"})
    t0 = time.time()
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            status, raw = resp.status, resp.read()
    except urllib.error.HTTPError as e:
        status, raw = e.code, e.read()
    except Exception as e:  # timeout / connection reset
        return -1, {"_error": repr(e)}, int((time.time() - t0) * 1000)
    ms = int((time.time() - t0) * 1000)
    try:
        return status, json.loads(raw), ms
    except ValueError:
        return status, {"_raw": raw[:2000].decode("utf-8", "replace")}, ms


def strip_echo(obj):
    if isinstance(obj, dict):
        return {k: strip_echo(v) for k, v in obj.items() if k not in ECHO_KEYS}
    if isinstance(obj, list):
        return [strip_echo(v) for v in obj]
    return obj


def result_rows(resp):
    ent = resp.get("enterprise") or {}
    rows = ((ent.get("data_grid") or {}).get("rows")) or []
    if not rows:
        rows = (resp.get("records") or {}).get("source_native_results") or []
    return rows if isinstance(rows, list) else []


def scalars(obj, skip=ROW_NOISE_KEYS):
    out = []
    if isinstance(obj, dict):
        for k, v in obj.items():
            if k not in skip:
                out.extend(scalars(v, skip))
    elif isinstance(obj, list):
        for v in obj:
            out.extend(scalars(v, skip))
    elif obj is not None:
        out.append(obj)
    return out


def as_number(v):
    if isinstance(v, bool):
        return None
    if isinstance(v, (int, float)):
        return float(v)
    if isinstance(v, str):
        s = v.replace(",", "").strip()
        if re.fullmatch(r"-?\d+(\.\d+)?", s):
            return float(s)
    return None


def answer_text(resp):
    ent = resp.get("enterprise") or {}
    narrative = ent.get("narrative") or {}
    parts = [ent.get("executive_answer") or "", narrative.get("direct_answer") or "",
             ent.get("summary") or "", (resp.get("answer") or {}).get("llm_summary") or ""]
    return "\n".join(p for p in parts if isinstance(p, str))


def number_stated(text, value):
    v = float(value)
    forms = {str(int(v))} if v.is_integer() else {str(v)}
    if v.is_integer():
        forms.add(f"{int(v):,}")
    return any(re.search(r"(?<![\d.,])" + re.escape(f) + r"(?![\d])", text) for f in forms)


def grade(item, status, resp, ms):
    ent = resp.get("enterprise") or {}
    state = str(ent.get("result_state") or "").lower()
    clar = ent.get("clarification") or ""
    rows = result_rows(resp)
    row_vals = scalars(rows)
    blob = json.dumps(strip_echo(copy.deepcopy(resp)), ensure_ascii=False)
    text = answer_text(resp)
    check, exp = item["check"], item.get("expect")
    verdict, stated = "UNGRADED", None

    if status != 200:
        verdict = "ERROR"
    elif check == "number":
        nums = [as_number(v) for v in row_vals]
        hit = any(n is not None and abs(n - float(exp)) < 1e-6 for n in nums)
        if hit:
            verdict = "CORRECT"
        elif clar:
            verdict = "CLARIFIED"
        elif len(rows) == exp:
            verdict = "ROWCOUNT_ONLY"
        else:
            verdict = "WRONG"
        stated = number_stated(text, exp)
    elif check == "top":
        if rows and exp in [str(v) for v in scalars(rows[0])]:
            verdict = "CORRECT"
        elif clar:
            verdict = "CLARIFIED"
        elif exp in blob:
            verdict = "PRESENT_NOT_TOP"
        else:
            verdict = "WRONG"
        stated = exp in text
    elif check == "contains":
        missing = [e for e in exp if str(e) not in blob]
        verdict = "CORRECT" if not missing else ("CLARIFIED" if clar else "WRONG")
        stated = all(str(e) in text for e in exp)
        if missing:
            item = dict(item, _missing=missing)
    elif check == "empty":
        zero = any(as_number(v) == 0 for v in row_vals)
        verdict = "CORRECT" if (not rows or zero or state in NON_ANSWER_STATES or clar) else "WRONG"
    elif check == "unsupported":
        verdict = "CORRECT" if (not rows or state in NON_ANSWER_STATES or clar) else "WRONG"
    elif check == "clarify":
        verdict = "CORRECT" if clar else "WRONG"
    elif check == "qualitative":
        verdict = "MANUAL"

    tel = resp.get("telemetry") or {}
    ans = resp.get("answer") or {}
    return {
        "id": item["id"], "family": item["family"], "collection": item["c"], "question": item["q"],
        "check": check, "expect": exp, "verdict": verdict, "stated_in_answer": stated,
        "missing": item.get("_missing"), "http": status, "latency_ms": ms,
        "llm_latency_ms": tel.get("llm_latency_ms"), "template": resp.get("template"),
        "route": resp.get("route"), "result_state": state, "clarification": clar[:300],
        "row_count": len(rows), "llm_summary": bool(ans.get("llm_summary")),
        "llm_fallback_reason": ans.get("llm_fallback_reason"),
        "answer_text": text[:600], "error": resp.get("error") or resp.get("_error") or resp.get("_raw"),
        "note": item.get("note"),
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--golden", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--base", default=os.environ.get("NX_BASE", "http://localhost:8091"))
    ap.add_argument("--only", default="", help="comma-separated ids")
    ap.add_argument("--timeout", type=int, default=240)
    args = ap.parse_args()

    env = load_env()
    golden = json.load(open(args.golden, encoding="utf-8"))
    cols = golden["collections"]
    only = set(filter(None, args.only.split(",")))
    os.makedirs(args.out, exist_ok=True)
    raw_dir = os.path.join(args.out, "raw")
    os.makedirs(raw_dir, exist_ok=True)

    results = []
    for item in golden["questions"]:
        if only and item["id"] not in only:
            continue
        status, resp, ms = post(args.base, env, cols[item["c"]], item["q"], args.timeout)
        with open(os.path.join(raw_dir, item["id"] + ".json"), "w", encoding="utf-8") as fh:
            json.dump(resp, fh, ensure_ascii=False, indent=1)
        r = grade(item, status, resp, ms)
        results.append(r)
        print(f"{r['id']:8} {r['verdict']:16} stated={str(r['stated_in_answer']):5} "
              f"{r['latency_ms']:>7}ms tpl={r['template']} state={r['result_state']}", flush=True)

    with open(os.path.join(args.out, "results.json"), "w", encoding="utf-8") as fh:
        json.dump(results, fh, ensure_ascii=False, indent=1)
    summary = {}
    for r in results:
        summary.setdefault(r["verdict"], 0)
        summary[r["verdict"]] += 1
    print(json.dumps(summary, indent=1))


if __name__ == "__main__":
    sys.exit(main())
