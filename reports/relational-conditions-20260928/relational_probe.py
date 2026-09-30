# A1c RELATIONAL-CONDITION PROBE -- not an evaluation suite.
#
# None of the 103 corpus questions states a relationship between records, so
# the corpus cannot measure this guard's benefit and can only show it is inert.
# This probe asks relational questions (R*) and near-miss questions that share
# a word with them but state no relationship (N*), once per arm.
#
# Read-only. Posts to the running API directly. No data is written, nothing is
# reprocessed.
#
#     python relational_probe.py <arm-name>
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

PROBES = [
    # Relational: a count or a one-kind listing is NOT an answer to these.
    ("R1", "Do any subscribers share the same handset?"),
    ("R2", "Is there any link between the plate sightings and the call records?"),
    ("R3", "Which phone numbers shared a cell tower?"),
    ("R4", "Are there subscribers sharing one IMEI?"),
    ("R5", "What do the two busiest numbers have in common?"),
    ("R6", "Is there any overlap between the CDR files?"),
    ("R7", "What is the connection between the call records and the internet sessions?"),
    ("R8", "Do any plates share the same camera?"),
    # Near-miss: same words, no relationship. Must be UNCHANGED by the switch.
    ("N1", "What is the share of SMS among all calls?"),
    ("N2", "Can you share a summary of the call records?"),
    ("N3", "Share a list of the cell sites"),
    ("N4", "How many subscribers are there?"),
    ("N5", "Which cell site handled the most calls?"),
    ("N6", "Which subscribers are linked to 923001110001?"),
    ("N7", "Show me all the calls that lasted longer than ten minutes"),
    ("N8", "How many ANPR sightings are there?"),
]


def ask(question, timeout=420):
    payload = json.dumps({"tenant_id": "default", "collection_id": DEMO,
                          "query": question}).encode("utf-8")
    request = urllib.request.Request(URL, data=payload, method="POST")
    for header, value in (("Content-Type", "application/json"),
                          ("X-Forensic-Tenant-ID", "default"),
                          ("X-Forensic-Collection-ID", DEMO),
                          ("X-Forensic-Actor-ID", "investigation-workspace"),
                          ("X-Forensic-Subject-ID", "investigation-workspace"),
                          ("X-Forensic-Actor-Role", "user")):
        request.add_header(header, value)
    if API_KEY:
        request.add_header("Authorization", "Bearer " + API_KEY)
    started = time.time()
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return json.loads(response.read().decode("utf-8")), time.time() - started
    except urllib.error.HTTPError as exc:
        return {"_http_error": exc.code, "_body": exc.read().decode("utf-8")[:200]}, time.time() - started
    except Exception as exc:  # noqa: BLE001 - a probe reports its own failure
        return {"_error": str(exc)[:200]}, time.time() - started


def walk_values(node, key):
    found = []
    if isinstance(node, dict):
        for name, value in node.items():
            if name == key and isinstance(value, str):
                found.append(value)
            found.extend(walk_values(value, key))
    elif isinstance(node, list):
        for item in node:
            found.extend(walk_values(item, key))
    return found


def classify(blob):
    # Same extractor as reports/adhoc-runtime-20260927/adhoc_probe.py, which was
    # proved on a known-good answer before it was trusted.
    if blob.get("_http_error") or blob.get("_error"):
        return "ERROR", blob.get("_body") or blob.get("_error") or ""
    text = json.dumps(blob)
    answer = blob.get("answer") if isinstance(blob.get("answer"), dict) else {}
    clarification = (answer.get("clarification") or "").strip()
    stated = next((value for value in walk_values(blob, "direct_answer") if value.strip()), "")
    uncomputed = '"executive_answer_uncomputed": true' in text
    if stated:
        return ("ANSWERED_UNCOMPUTED" if uncomputed else "ANSWERED"), stated
    if clarification:
        return "CLARIFIED", clarification
    if uncomputed:
        return "UNCOMPUTED", "executive answer not computed"
    return "EMPTY", json.dumps(answer)[:200]


def plan_summary(blob):
    planner = blob.get("planner") or {}
    applied = ((planner.get("query_plan") or {}).get("applied_filters") or {})
    plan = applied.get("source_native") or {}
    return {
        "record_type": applied.get("record_type"),
        "group_fields": plan.get("group_fields"),
        "measures": [m.get("op") for m in (plan.get("measures") or [])],
        "having": plan.get("having"),
        "language_assistance": planner.get("language_assistance"),
    }


def selftest():
    blob, _ = ask("How many CDR records do we have in this case?")
    state, text = classify(blob)
    ok = state.startswith("ANSWERED") and "8,642" in text
    print("SELFTEST control question -> %s : %s" % (state, text[:90]))
    if not ok:
        print("SELFTEST FAILED - the extractor cannot see a known-good answer. No verdict is reported.")
    return ok


def main():
    if len(sys.argv) != 2:
        print("usage: relational_probe.py <arm-name>")
        return 2
    arm = sys.argv[1]
    if not selftest():
        return 1
    out_dir = os.path.join(HERE, arm)
    os.makedirs(os.path.join(out_dir, "raw"), exist_ok=True)
    rows = []
    for probe_id, question in PROBES:
        blob, seconds = ask(question)
        state, text = classify(blob)
        clarification = blob.get("clarification") if isinstance(blob.get("clarification"), dict) else {}
        row = {"id": probe_id, "question": question, "state": state, "text": text[:400],
               "reason_code": clarification.get("reason_code"), "template": blob.get("template"),
               "route": blob.get("route"), "plan": plan_summary(blob), "seconds": round(seconds, 1)}
        rows.append(row)
        io.open(os.path.join(out_dir, "raw", probe_id + ".json"), "w", encoding="utf-8").write(json.dumps(blob, indent=1))
        print("[%s] %-19s %5.1fs  %s" % (probe_id, state, seconds, question))
        print("     template=%s reason=%s plan=%s" % (row["template"], row["reason_code"], json.dumps(row["plan"])))
        print("     says: %s" % (text[:200].replace("\n", " ") or "(nothing)"))
        sys.stdout.flush()
    io.open(os.path.join(out_dir, "results.json"), "w", encoding="utf-8").write(json.dumps(rows, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
