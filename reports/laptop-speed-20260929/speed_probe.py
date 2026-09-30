# LAPTOP SPEED PROBE -- wall time per question on questions outside the corpus.
#
# Read-only: posts questions to the running API and records the time, route and answer text.
#
#     python reports/laptop-speed-20260929/speed_probe.py <arm>
import io
import json
import os
import sys
import time
import urllib.error
import urllib.request

URL = os.environ.get("NEXUSAI_PROBE_URL", "http://127.0.0.1:8091/query/hybrid")
API_KEY = os.environ.get("FORENSIC_RECORDS_API_KEY", "")
DEMO = "nexusai-forensic-demo"
HERE = os.path.dirname(os.path.abspath(__file__))

# Grouped by evidence type so consecutive questions can reuse a cached prompt start.
QUESTIONS = [
    ("S1", "What is the total duration in seconds of all calls made by 923001110001?"),
    ("S2", "How many SMS events are recorded in the call data?"),
    ("S3", "Which cell site id appears most often in the CDRs?"),
    ("S4", "What is the longest single call duration recorded in the call records?"),
    ("S5", "How many ANPR sightings are there for plate LHR-2026 in total?"),
    ("S6", "Which ANPR camera recorded the most sightings?"),
    ("S7", "How many IPDR sessions used the HTTPS protocol?"),
    ("S8", "What is the total number of bytes uploaded across all IPDR sessions?"),
]


def ask(question, timeout=600):
    payload = json.dumps({"tenant_id": "default", "collection_id": DEMO, "query": question}).encode("utf-8")
    request = urllib.request.Request(URL, data=payload, method="POST")
    for header, value in (("Content-Type", "application/json"), ("X-Forensic-Tenant-ID", "default"),
                          ("X-Forensic-Collection-ID", DEMO), ("X-Forensic-Actor-ID", "investigation-workspace"),
                          ("X-Forensic-Subject-ID", "investigation-workspace"), ("X-Forensic-Actor-Role", "user")):
        request.add_header(header, value)
    if API_KEY:
        request.add_header("Authorization", "Bearer " + API_KEY)
    started = time.time()
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return json.loads(response.read().decode("utf-8")), time.time() - started
    except urllib.error.HTTPError as exc:
        return {"_http_error": exc.code}, time.time() - started
    except Exception as exc:  # noqa: BLE001 - a probe reports its own failure
        return {"_error": str(exc)[:200]}, time.time() - started


def main():
    arm = sys.argv[1] if len(sys.argv) > 1 else "run"
    out = os.path.join(HERE, arm)
    os.makedirs(out, exist_ok=True)
    rows = []
    for qid, question in QUESTIONS:
        blob, seconds = ask(question)
        enterprise = blob.get("enterprise") if isinstance(blob.get("enterprise"), dict) else {}
        sop = ((blob.get("planner") or {}).get("semantic_operation_planner") or {})
        row = {"id": qid, "question": question, "seconds": round(seconds, 1), "route": blob.get("route"),
               "template": blob.get("template"), "ir_outcome": sop.get("ir_fallback_outcome"),
               "ir_cache_hit": sop.get("ir_plan_cache_hit"), "ir_generation_ms": sop.get("ir_generation_ms"),
               "text": (enterprise.get("executive_answer") or "")[:500], "error": blob.get("_error") or blob.get("_http_error")}
        rows.append(row)
        print("%s %6.1fs  ir=%s cache=%s  %s" % (qid, seconds, row["ir_outcome"], row["ir_cache_hit"], row["text"][:110]))
    with io.open(os.path.join(out, "results.json"), "w", encoding="utf-8") as handle:
        json.dump(rows, handle, indent=1, ensure_ascii=False)
    times = sorted(r["seconds"] for r in rows)
    print("median %.1fs  total %.1fs" % (times[len(times) // 2], sum(times)))


if __name__ == "__main__":
    main()
