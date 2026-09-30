#!/usr/bin/env python3
"""Ask the live API arbitrary questions and show how it classified and answered.

The golden harness scores questions against expectations. This one exists for
the questions that have no expectation — greetings, product help, domain
concepts, out-of-scope — where the only thing that matters is whether the
response is APPROPRIATE and whether it invents capability.

    python scripts/nexusai_probe.py "hi" "what is an IMSI" "how do I add evidence"
    python scripts/nexusai_probe.py --file probes.txt

Prints, per question: request class, intent, route, template, and the
analyst-facing answer. Read-only; it posts queries exactly as the UI does.
"""

import json
import os
import sys
import time
import urllib.error
import urllib.request

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BASE = os.environ.get("NX_BASE", "http://localhost:8091")
COLLECTION = os.environ.get("NX_COLLECTION", "nexusai-forensic-demo")


def load_env():
    values = {}
    path = os.path.join(REPO, ".env.forensic-runtime.local")
    with open(path, encoding="utf-8") as handle:
        for line in handle:
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                key, value = line.split("=", 1)
                values[key] = value.strip().strip('"')
    return values


def ask(env, question, timeout=300):
    body = json.dumps({
        "tenant_id": env.get("FORENSIC_RECORDS_TENANT_ID", "default"),
        "collection_id": COLLECTION,
        "query": question,
    }).encode()
    request = urllib.request.Request(BASE + "/query/hybrid", data=body, method="POST", headers={
        "Authorization": "Bearer " + env["FORENSIC_RECORDS_API_KEY"],
        "X-Forensic-Tenant-ID": env.get("FORENSIC_RECORDS_TENANT_ID", "default"),
        "X-Forensic-Actor-ID": "probe", "X-Forensic-Subject-ID": "probe",
        "X-Forensic-Actor-Role": "admin", "X-Forensic-Collection-ID": COLLECTION,
        "Content-Type": "application/json"})
    started = time.time()
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            status, raw = response.status, response.read()
    except urllib.error.HTTPError as err:
        status, raw = err.code, err.read()
    except Exception as err:  # noqa: BLE001 - a probe reports failures, never raises
        return {"_error": str(err)}, 0, int((time.time() - started) * 1000)
    try:
        payload = json.loads(raw.decode("utf-8", "replace"))
    except Exception:  # noqa: BLE001
        payload = {"_unparseable": raw.decode("utf-8", "replace")[:400]}
    return payload, status, int((time.time() - started) * 1000)


def analyst_text(payload):
    answer = payload.get("answer") or {}
    # `answer.answer` FIRST. An earlier version of this probe omitted it and
    # reported "(no analyst-facing text)" for PRODUCT_HELP and
    # GENERAL_DOMAIN_KNOWLEDGE -- both of which were answering correctly. The
    # probe was the defect, not the product.
    for key in ("answer", "analyst_answer", "answer_text", "summary", "clarification", "explanation"):
        value = answer.get(key)
        if isinstance(value, str) and value.strip():
            return value.strip()
    enterprise = payload.get("enterprise") or {}
    for key in ("analyst_answer", "executive_answer", "summary"):
        value = enterprise.get(key)
        if isinstance(value, str) and value.strip():
            return value.strip()
    return "(no analyst-facing text)"


def main(argv):
    questions = []
    if len(argv) > 2 and argv[1] == "--file":
        with open(argv[2], encoding="utf-8") as handle:
            questions = [line.strip() for line in handle if line.strip()]
    else:
        questions = argv[1:]
    if not questions:
        print(__doc__)
        return 2

    env = load_env()
    for question in questions:
        payload, status, ms = ask(env, question)
        print("=" * 78)
        print("Q: %s" % question)
        if "_error" in payload:
            print("   TRANSPORT ERROR: %s" % payload["_error"])
            continue
        print("   http=%s  %dms  class=%s  intent=%s  route=%s  template=%s" % (
            status, ms,
            payload.get("request_class"), payload.get("intent"),
            ",".join(payload.get("route") or []) or "-",
            payload.get("template") or "-"))
        text = analyst_text(payload)
        print("   A: %s" % (text[:500].replace("\n", " ")))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
