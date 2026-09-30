# AD-HOC RUNTIME PROBE -- not an evaluation suite.
#
# Fourteen questions written NOW, deliberately not copied from golden_questions_v2,
# holdout_questions_v1 or holdout_media_v1, and phrased the way a person asks
# rather than the way a template was written. The point is to answer one
# question honestly: what happens when somebody types their own words?
#
# Read-only. Posts to the running API through the workspace proxy. No data is
# written, nothing is reprocessed, no suite is run.
import io
import json
import os
import re
import sys
import time
import urllib.error
import urllib.request

# Default to the API DIRECTLY, not the workspace dev proxy: a measurement must
# not depend on a UI dev server being up. The 2026-09-27 run failed its selftest
# for exactly that reason -- correctly, and it reported nothing rather than
# reporting a working system as broken.
PROXY = os.environ.get("NEXUSAI_PROBE_URL", "http://127.0.0.1:8091/query/hybrid")
API_KEY = os.environ.get("FORENSIC_RECORDS_API_KEY", "")
DEMO = "nexusai-forensic-demo"
MM = "nexusai-multimodal-product-acceptance"

PROBES = [
    # (label, collection, question)
    ("structured/rephrased", DEMO, "Who did that number 923001110001 speak to most often?"),
    ("structured/filter-list", DEMO, "Show me all the calls that lasted longer than ten minutes"),
    ("structured/vague-time", DEMO, "What happened on 15 June 2026?"),
    ("structured/relational", DEMO, "Do any subscribers share the same handset?"),
    ("structured/cross-family", DEMO, "Is there any link between the plate sightings and the call records?"),
    ("structured/open-ended", DEMO, "Summarise the suspicious activity in this case"),
    ("media/open-ended", MM, "What can you tell me about the video evidence?"),
    ("media/rephrased", MM, "How many of the images have faces in them?"),
    ("media/list", MM, "List the audio files in this case"),
    ("media/known-gap", MM, "What languages were detected in the audio?"),
    ("media/entity-centric", MM, "Show me everything you have about plate ABC-123"),
    ("conversational/greeting", DEMO, "hello"),
    ("conversational/capability", DEMO, "What can this system do?"),
    ("conversational/concept", DEMO, "What is a CDR?"),
]


def ask(collection, question, timeout=420):
    payload = json.dumps({"tenant_id": "default", "collection_id": collection,
                          "query": question}).encode("utf-8")
    request = urllib.request.Request(PROXY, data=payload, method="POST")
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
    except Exception as exc:  # noqa: BLE001 - a probe reports its own failure
        return {"_error": str(exc)[:200]}, time.time() - started


def walkValues(node, key):
    """Every string stored under `key`, anywhere in the document."""
    found = []
    if isinstance(node, dict):
        for name, value in node.items():
            if name == key and isinstance(value, str):
                found.append(value)
            found.extend(walkValues(value, key))
    elif isinstance(node, list):
        for item in node:
            found.extend(walkValues(item, key))
    return found


def classify(blob):
    """What did the system DO -- answered, refused, or asked for more?

    `direct_answer` is NOT at a fixed path: the verified answers in this
    session (8,642 CDR / 1,057 ANPR) were found by searching the whole
    payload, and reading `answer.narrative.direct_answer` reported every one
    of them as empty. Search the serialised blob, the same way the check that
    was verified against a known-good answer does.
    """
    if blob.get("_http_error") or blob.get("_error"):
        return "ERROR", blob.get("_body") or blob.get("_error") or ""
    text = json.dumps(blob)
    answer = blob.get("answer") if isinstance(blob.get("answer"), dict) else {}
    clarification = (answer.get("clarification") or "").strip()
    # WALK the parsed document rather than regexing its serialisation. A
    # refusal embeds the analyst's own words with Go's %q, so the sentence
    # contains escaped quotes and `"([^"]*)"` stops at the first one -- which
    # truncated the A1a evidence to `I did not apply the condition \` on
    # 2026-09-27 and made a working guard look broken.
    stated = next((value for value in walkValues(blob, "direct_answer") if value.strip()), "")
    uncomputed = '"executive_answer_uncomputed": true' in text
    if stated:
        return ("ANSWERED_UNCOMPUTED" if uncomputed else "ANSWERED"), stated
    if clarification:
        return "CLARIFIED", clarification
    if uncomputed:
        return "UNCOMPUTED", "executive answer not computed"
    return "EMPTY", json.dumps(answer)[:200]


def selftest():
    """Prove the instrument on a KNOWN-GOOD answer before trusting a verdict.

    The first version of this probe reported 0 of 14 answered, which was the
    extractor failing, not the product. Nothing below is reported unless this
    control question comes back ANSWERED with 8,642 in it.
    """
    blob, _ = ask(DEMO, "How many CDR records do we have in this case?")
    state, text = classify(blob)
    ok = state.startswith("ANSWERED") and "8,642" in text
    print("SELFTEST control question -> %s : %s" % (state, text[:90]))
    if not ok:
        print("SELFTEST FAILED - the extractor cannot see a known-good answer. "
              "No verdict is reported.")
    print("")
    return ok


def main():
    if not selftest():
        return 1
    out = []
    for label, collection, question in PROBES:
        blob, seconds = ask(collection, question)
        state, text = classify(blob)
        row = {"label": label, "collection": collection, "question": question,
               "state": state, "text": text[:400],
               "request_class": blob.get("request_class"),
               "template": blob.get("template"), "seconds": round(seconds, 1)}
        out.append(row)
        print("[%-24s] %-10s %5.1fs  %s" % (label, state, seconds, question))
        print("    class=%s template=%s" % (row["request_class"], row["template"]))
        print("    says: %s" % (text[:230].replace("\n", " ") or "(nothing)"))
        sys.stdout.flush()
    path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "adhoc_results.json")
    io.open(path, "w", encoding="utf-8").write(json.dumps(out, indent=2))
    print("")
    counts = {}
    for row in out:
        counts[row["state"]] = counts.get(row["state"], 0) + 1
    print("TOTALS: %s" % " | ".join("%s %d" % kv for kv in sorted(counts.items())))
    return 0


if __name__ == "__main__":
    sys.exit(main())
