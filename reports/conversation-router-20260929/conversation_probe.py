# B1 CONVERSATION PROBE -- greetings, capability questions, glossary, and the questions that must NOT move.
#
# Reads the analyst-facing `enterprise.executive_answer`, which is where a terminal (non-evidence) answer
# is shown. The everyday probe reads only `narrative.direct_answer`, so it reports terminal answers as
# EMPTY; it is left unchanged so its earlier runs stay comparable.
#
# Read-only. Posts to the running API. Nothing is written.
#
#     python reports/conversation-router-20260929/conversation_probe.py <arm>
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
MM = "nexusai-multimodal-product-acceptance"
HERE = os.path.dirname(os.path.abspath(__file__))

# (id, collection, question, kind, must-contain)
#   help      terminal product help, text contains the fragment
#   define    terminal definition, text contains the fragment
#   analysis  must NOT be terminal (it asks about evidence); fragment optional
PROBES = [
    ("G1", DEMO, "hello", "help", "Hello. I answer questions about the evidence"),
    ("G2", DEMO, "Hi there!", "help", "Hello. I answer questions about the evidence"),
    ("G3", DEMO, "Assalam o Alaikum", "help", "Hello. I answer questions about the evidence"),
    ("C1", DEMO, "What can this system do?", "help", "call detail records"),
    ("C2", DEMO, "What kinds of questions can I ask?", "help", "I say so instead of guessing"),
    ("C3", DEMO, "help", "help", "ANPR and vehicle sightings"),
    ("D1", DEMO, "What is a CDR?", "define", "A CDR (call detail record)"),
    ("D2", DEMO, "What does ANPR mean?", "define", "ANPR (automatic number plate recognition)"),
    ("D3", DEMO, "What is a LAC?", "define", "A LAC (location area code)"),
    ("D4", DEMO, "What is a CNIC?", "define", "CNIC is Pakistan's"),
    ("D5", DEMO, "What is storage?", "define", "A bounded general definition is unavailable"),
    ("N1", DEMO, "hello, how many CDR records are in this case?", "analysis", ""),
    ("N2", MM, "What can you tell me about the video evidence?", "analysis", ""),
    ("N3", DEMO, "How many CDR records are in this case?", "analysis", "8,642"),
    ("N4", DEMO, "Who did 923001110001 contact most frequently?", "analysis", "923009998887"),
]


def ask(collection, question, timeout=420):
    payload = json.dumps({"tenant_id": "default", "collection_id": collection, "query": question}).encode("utf-8")
    request = urllib.request.Request(URL, data=payload, method="POST")
    for header, value in (("Content-Type", "application/json"), ("X-Forensic-Tenant-ID", "default"),
                          ("X-Forensic-Collection-ID", collection), ("X-Forensic-Actor-ID", "investigation-workspace"),
                          ("X-Forensic-Subject-ID", "investigation-workspace"), ("X-Forensic-Actor-Role", "user")):
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


def main():
    arm = sys.argv[1] if len(sys.argv) > 1 else "run"
    out = os.path.join(HERE, arm)
    os.makedirs(os.path.join(out, "raw"), exist_ok=True)
    results = []
    for pid, collection, question, kind, fragment in PROBES:
        blob, seconds = ask(collection, question)
        with io.open(os.path.join(out, "raw", pid + ".json"), "w", encoding="utf-8") as handle:
            json.dump(blob, handle, indent=1, ensure_ascii=False, default=str)
        enterprise = blob.get("enterprise") if isinstance(blob.get("enterprise"), dict) else {}
        text = (enterprise.get("executive_answer") or "").strip()
        route = blob.get("route") or []
        terminal = route == ["terminal"]
        if blob.get("_http_error") or blob.get("_error"):
            ok = False
        elif kind in ("help", "define"):
            ok = terminal and fragment in text
        else:
            ok = not terminal and (fragment in text if fragment else True)
        results.append({"id": pid, "question": question, "kind": kind, "pass": ok, "route": route,
                        "request_class": blob.get("request_class"), "template": blob.get("template"),
                        "text": text[:600], "seconds": round(seconds, 1)})
        print("%s %-4s %-8s %-28s %s" % ("PASS" if ok else "FAIL", pid, kind, str(blob.get("request_class"))[:28], question))
        print("      %s" % text[:200].replace("\n", " "))
    with io.open(os.path.join(out, "results.json"), "w", encoding="utf-8") as handle:
        json.dump(results, handle, indent=1, ensure_ascii=False)
    print("%d/%d pass" % (sum(r["pass"] for r in results), len(results)))


if __name__ == "__main__":
    main()
