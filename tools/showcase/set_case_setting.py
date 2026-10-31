#!/usr/bin/env python3
"""Shows or sets the settings of a case record on a RUNNING forensic API: the display name, the status and the hub threshold (migration 032).

    python -X utf8 tools/showcase/set_case_setting.py --case orion-lahore-reference-20261030                      show the case record
    python -X utf8 tools/showcase/set_case_setting.py --case orion-lahore-reference-20261030 --hub-threshold 10 --apply   set it (2 to 1000)
    python -X utf8 tools/showcase/set_case_setting.py --case orion-lahore-reference-20261030 --clear-hub-threshold --apply  back to the engine default (20)

Without --apply nothing is written: the request that would be sent is printed. The API key is read from the main checkout's .env.forensic-runtime.local and never printed. The change is audited by the API
(one row in records_audit_log, the case's own record). The threshold is what the Ask engine uses for "numbers and services that everyone contacts" when the question names no number of its own.
"""
import argparse
import json
import os
import sys
import urllib.error
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
MAIN = os.path.join(os.path.dirname(os.path.abspath(os.path.join(HERE, "..", ".."))), "NexusAI")


def api_key():
    with open(os.path.join(MAIN, ".env.forensic-runtime.local"), encoding="utf-8") as handle:
        for line in handle:
            if line.startswith("FORENSIC_RECORDS_API_KEY="):
                return line.split("=", 1)[1].strip().strip('"')
    raise SystemExit("FORENSIC_RECORDS_API_KEY is not in the env file of the main checkout")


def call(url, case, method, body=None):
    request = urllib.request.Request("%s/cases/%s?tenant_id=default" % (url.rstrip("/"), case), data=json.dumps(body).encode("utf-8") if body is not None else None, method=method)
    for header, value in (("Content-Type", "application/json"), ("Authorization", "Bearer " + api_key()), ("X-Forensic-Tenant-ID", "default"), ("X-Forensic-Collection-ID", case),
                          ("X-Forensic-Actor-ID", "case-setting-tool"), ("X-Forensic-Subject-ID", "case-setting-tool"), ("X-Forensic-Actor-Role", "user")):
        request.add_header(header, value)
    try:
        with urllib.request.urlopen(request, timeout=60) as response:
            return response.status, json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as error:
        return error.code, json.loads(error.read().decode("utf-8", "replace") or "{}")


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--case", required=True)
    parser.add_argument("--url", default="http://127.0.0.1:8091")
    parser.add_argument("--hub-threshold", type=int)
    parser.add_argument("--clear-hub-threshold", action="store_true")
    parser.add_argument("--apply", action="store_true")
    args = parser.parse_args(argv)
    body = {}
    if args.hub_threshold is not None:
        body["hub_threshold"] = args.hub_threshold
    if args.clear_hub_threshold:
        body["hub_threshold"] = None
    if body:
        print("request: PATCH /cases/%s %s" % (args.case, json.dumps(body)))
        if not args.apply:
            print("dry run: nothing was written (add --apply)")
            return 0
        status, data = call(args.url, args.case, "PATCH", body)
    else:
        status, data = call(args.url, args.case, "GET")
    print("HTTP %s" % status)
    print(json.dumps(data, indent=1, ensure_ascii=False))
    return 0 if status == 200 else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
