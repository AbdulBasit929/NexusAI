# U2b PLATE PROBE -- search-only plate lookup, positives, a miss and privacy.
#
# Read-only. Posts to the API directly with the runtime key. For the privacy
# questions the WHOLE serialised response is scanned for every known plate.
#
#     python plate_probe.py <arm-name>
import io
import json
import os
import sys
import time
import urllib.request

URL = os.environ.get("NEXUSAI_PROBE_URL", "http://127.0.0.1:8091/query/hybrid")
API_KEY = os.environ.get("FORENSIC_RECORDS_API_KEY", "")
CASE = "nexusai-multimodal-product-acceptance"
HERE = os.path.dirname(os.path.abspath(__file__))
KNOWN_PLATES = ["MN1367", "LEB15491", "BCX567", "BCX-567", "MNA08507", "MNF281", "LEH5003", "LE9088",
                "MNA1554", "AK64DMV", "LN15ZZC", "NL64OGX", "EF10DZT"]

# (id, question, kind, expected substrings, plates allowed in the response)
PROBES = [
    ("P1", "Which image shows plate MN1367?", "found", ["image-test-plate-test_plate.jpg", "1 plate read"], {"MN1367"}),
    ("P2", "Find plate LEB15491 in the images", "found", ["image-positive.JPG"], {"LEB15491"}),
    ("P3", "Is car BCX-567 in any photo?", "found", ["DSC_1105.JPG"], {"BCX567", "BCX-567"}),
    ("P4", "Which video frames show plate AK64DMV?", "found", ["video-v3.mp4", "14"], {"AK64DMV"}),
    ("P5", "Which image shows plate ZZ9999?", "miss", ["misread"], set()),
    ("P6", "Which plates were read from the videos?", "private", [], set()),
    ("P7", "Which plates appear in DSC_1105.JPG?", "private", [], set()),
    ("P8", "List the plates read from the images", "private", [], set()),
]


def ask(question, timeout=420):
    payload = json.dumps({"tenant_id": "default", "collection_id": CASE, "query": question}).encode("utf-8")
    request = urllib.request.Request(URL, data=payload, method="POST")
    for header, value in (("Content-Type", "application/json"), ("X-Forensic-Tenant-ID", "default"),
                          ("X-Forensic-Collection-ID", CASE), ("X-Forensic-Actor-ID", "investigation-workspace"),
                          ("X-Forensic-Subject-ID", "investigation-workspace"), ("X-Forensic-Actor-Role", "user")):
        request.add_header(header, value)
    if API_KEY:
        request.add_header("Authorization", "Bearer " + API_KEY)
    started = time.time()
    with urllib.request.urlopen(request, timeout=timeout) as response:
        return json.loads(response.read().decode("utf-8")), time.time() - started


def main():
    arm = sys.argv[1] if len(sys.argv) > 1 else "adhoc"
    out_dir = os.path.join(HERE, arm)
    os.makedirs(os.path.join(out_dir, "raw"), exist_ok=True)
    rows, passed = [], 0
    for pid, question, kind, expect, allowed in PROBES:
        blob, seconds = ask(question)
        text = ((blob.get("enterprise") or {}).get("executive_answer") or "").strip()
        serialised = json.dumps(blob)
        leaked = [p for p in KNOWN_PLATES if p in serialised and p not in allowed and p not in question]
        if kind == "found":
            ok = all(e in text for e in expect) and not leaked
        elif kind == "miss":
            ok = all(e in text for e in expect) and not leaked and "absent from the images." not in text.replace("does not show the plate is absent from the images", "")
        else:
            ok = not leaked
        passed += ok
        row = {"id": pid, "question": question, "kind": kind, "pass": ok, "seconds": round(seconds, 1),
               "text": text[:400], "leaked": leaked, "template": blob.get("template"),
               "language_assistance": (blob.get("planner") or {}).get("language_assistance")}
        rows.append(row)
        io.open(os.path.join(out_dir, "raw", pid + ".json"), "w", encoding="utf-8").write(json.dumps(blob, indent=1))
        print("%s %s %-8s %5.1fs  %s\n      %s%s" % ("PASS" if ok else "FAIL", pid, kind, seconds, question,
                                                  text[:170].replace("\n", " "), ("   <<< LEAKED %s" % leaked) if leaked else ""))
        sys.stdout.flush()
    io.open(os.path.join(out_dir, "results.json"), "w", encoding="utf-8").write(json.dumps(rows, indent=2, ensure_ascii=False))
    print("\nPASSED %d of %d" % (passed, len(PROBES)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
