# DEMO PRE-FLIGHT -- not an evaluation suite.
#
# Every question offered for the 2026-09-28 team-lead demo, asked of the
# running API exactly as a person would type it, with the answer it must give.
# A question that fails here is taken OFF the demo list, not argued with.
# The plate-search rows are exploratory: they establish which wordings reach
# the image evidence at all.
#
# Read-only. Posts to the API directly with the runtime key.
#
#     python preflight.py
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
# Copy of reports/count-names-field-20260929/on/preflight/preflight.py (the 38-question pre-flight, shipped posture) made 2026-10-02 so the front-door arms
# do not overwrite that arm's saved results. Only change: the output folder comes from NEXUSAI_OUT.
OUT = os.environ.get("NEXUSAI_OUT", HERE)

# (group, case, question, kind, expected substrings)
#   kind "answer"   -> must be stated, not withheld, and contain every expected substring
#   kind "withhold" -> must be withheld and contain every expected substring
CHECKS = [
    ("counts", DEMO, "How many CDR records do we have in this case?", "answer", ["8,642"]),
    ("counts", DEMO, "count the cdrs", "answer", ["8,642"]),
    ("counts", DEMO, "How many subscribers are active?", "answer", ["6"]),
    ("model", DEMO, "Which cell site handled the most calls?", "answer", ["149631808"]),
    ("model", DEMO, "Which subscriber has the most internet sessions?", "answer", ["923001110002"]),
    ("model", DEMO, "How many unique phone numbers appear as callers in the CDRs?", "answer", ["10"]),
    ("model", DEMO, "How many call records are from August 2026?", "answer", ["2 CDR records"]),
    ("model", DEMO, "What is the total number of bytes transferred in IPDR sessions?", "answer", ["9,764,124,834"]),
    ("model", DEMO, "Which vehicle was seen most often?", "answer", ["ABC-123"]),
    ("model", DEMO, "How many times was plate LHR-2026 seen?", "answer", ["87"]),
    ("model", DEMO, "Which camera recorded the most sightings?", "answer", ["CAM-12"]),
    ("model", DEMO, "What was the largest transaction?", "answer", ["75,000"]),
    ("model", DEMO, "What is the total transaction amount?", "answer", ["129,700"]),
    ("model", DEMO, "What date range do the CDR records cover?", "answer", ["2026-04-01", "2026-08-10"]),
    ("breakdown", DEMO, "Show the call type breakdown", "answer", ["Data session: 5,863"]),
    ("breakdown", DEMO, "Show the breakdown of HTTP status codes in the access logs", "answer", ["697"]),
    ("breakdown", DEMO, "How many CDR records came from each source file?", "answer", ["seed_cdr_large.csv: 5,000"]),
    ("breakdown", DEMO, "Which domain was accessed most often?", "answer", ["chat.example.test"]),
    ("filter", DEMO, "How many transactions have an amount above 50000?", "answer", ["1 transaction", "above 50000"]),
    ("filter", DEMO, "How many IPDR sessions have bytes above 1000000?", "answer", ["2,189"]),
    ("refuse", DEMO, "Show me all the calls that lasted longer than ten minutes", "withhold", ["longer than ten minutes"]),
    ("refuse", DEMO, "Do any subscribers share the same handset?", "withhold", ["share the same handset"]),
    ("refuse", DEMO, "Where was 923001110001 seen according to the call records?", "withhold", []),
    ("refuse", DEMO, "How many emails are in this case?", "answer", ["no email records"]),
    ("refuse", DEMO, "Where was plate ZZZ-0000 seen?", "answer", ["No matching records"]),
    ("media", MM, "Which recording mentions coconut sugar and at what time?", "answer", ["11.28"]),
    ("media", MM, "Is the number 03001234567 mentioned in any audio?", "answer", ["urdu-english-identifier.wav"]),
    ("media", MM, "Which document mentions contact number 03001234567?", "answer", ["nexusai-multimodal-acceptance-brief.pdf"]),
    ("media", MM, "Find OCR text mentioning Investigation Workspace", "answer", ["printed-english.png"]),
    ("media", MM, "How many faces were detected in the evidence?", "answer", ["20"]),
    ("media", MM, "How many plate reads were produced from the images?", "answer", ["307"]),
    ("media", MM, "How many ANPR sightings are in this case?", "answer", ["1,057"]),
    ("plate-search", MM, "Find OCR text mentioning BCX-567", "answer", ["DSC_1105.JPG"]),
    ("plate-search", MM, "Find OCR text mentioning SINDH", "answer", ["DSC_1105.JPG"]),
    ("plate-search", MM, "Find OCR text mentioning MNA-08", "answer", ["DSC_0990.JPG"]),
    ("plate-search", MM, "Search image OCR for BCX-567", "answer", ["DSC_1105.JPG"]),
    ("plate-search", MM, "Which image shows plate MN1367?", "answer", ["image-test-plate-test_plate.jpg"]),
    ("plate-search", MM, "Find plate LEB15491 in the images", "answer", ["image-positive.JPG"]),
]


def ask(collection, question, timeout=420):
    payload = json.dumps({"tenant_id": "default", "collection_id": collection, "query": question}).encode("utf-8")
    request = urllib.request.Request(URL, data=payload, method="POST")
    for header, value in (("Content-Type", "application/json"),
                          ("X-Forensic-Tenant-ID", "default"),
                          ("X-Forensic-Collection-ID", collection),
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
    except Exception as exc:  # noqa: BLE001
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


def analyst_text(blob):
    stated = next((v for v in walk_values(blob, "direct_answer") if v.strip()), "")
    if stated:
        return stated
    answer = blob.get("answer") if isinstance(blob.get("answer"), dict) else {}
    ent = blob.get("enterprise") if isinstance(blob.get("enterprise"), dict) else {}
    return (answer.get("clarification") or ent.get("executive_answer") or "").strip()


def is_withheld(blob):
    route = blob.get("route") or []
    clar = blob.get("clarification") if isinstance(blob.get("clarification"), dict) else {}
    return bool(clar.get("reason_code")) or "verified_only_withheld" in route or "clarification" in route


def main():
    rows, passed = [], 0
    for group, case, question, kind, expect in CHECKS:
        blob, seconds = ask(case, question)
        text = analyst_text(blob)
        withheld = is_withheld(blob)
        if blob.get("_http_error") or blob.get("_error"):
            ok, why = False, "ERROR %s" % (blob.get("_http_error") or blob.get("_error"))
        elif kind == "answer":
            missing = [e for e in expect if e not in text]
            ok = not withheld and not missing
            why = "withheld" if withheld else ("missing " + ", ".join(missing) if missing else "")
        else:
            missing = [e for e in expect if e not in text]
            ok = withheld and not missing
            why = "not withheld" if not withheld else ("missing " + ", ".join(missing) if missing else "")
        passed += ok
        row = {"group": group, "case": case, "question": question, "kind": kind, "expect": expect,
               "pass": ok, "why": why, "seconds": round(seconds, 1), "withheld": withheld,
               "template": blob.get("template"), "route": blob.get("route"), "text": text[:400]}
        rows.append(row)
        print("%s %-12s %6.1fs  %s" % ("PASS" if ok else "FAIL", group, seconds, question))
        print("       %s%s" % (text[:160].replace("\n", " "), ("   <<< " + why) if why else ""))
        sys.stdout.flush()
    os.makedirs(OUT, exist_ok=True)
    io.open(os.path.join(OUT, "results.json"), "w", encoding="utf-8").write(json.dumps(rows, indent=2, ensure_ascii=False))
    print("\nPASSED %d of %d" % (passed, len(CHECKS)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
