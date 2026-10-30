#!/usr/bin/env python3
"""Uploads the files of a reference case (tools/showcase/build_showcase.py) to a RUNNING forensic API, one by one, the way an analyst's upload does. SYNTHETIC cases only.

    python -X utf8 tools/showcase/upload_case.py CASE_DIR --collection orion-lahore-20261030 [--include docs --include 'anpr*'] [--exclude statements] [--apply]

Without --apply it only lists what it would send. With --apply it POSTs each file to /webhooks/records/upload (multipart: file, collection_id, case_id, tenant_id, record_type=auto, the declared
time zone and day-first date order, skip_kb_mirror), with the API key read from the main checkout's .env.forensic-runtime.local (never printed). The files under CASE_DIR/files are taken in name
order; --include and --exclude match a file's path relative to files/ (a folder name or a glob). The device reference list (device-reference.csv) is tenant-wide reference data, not an upload of the
case, and is always left out.

Uploads are idempotent by content hash within a collection: sending a file twice reports it as a duplicate and queues nothing. A case folder is NEVER sent unless --apply says so.
"""
import argparse
import fnmatch
import json
import os
import sys
import urllib.error
import urllib.request
import uuid

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
MAIN = os.path.join(os.path.dirname(REPO), "NexusAI")
NEVER = {"device-reference.csv"}


def api_key():
    path = os.path.join(MAIN, ".env.forensic-runtime.local")
    with open(path, encoding="utf-8") as handle:
        for line in handle:
            if line.startswith("FORENSIC_RECORDS_API_KEY="):
                return line.split("=", 1)[1].strip()
    raise SystemExit("FORENSIC_RECORDS_API_KEY is not in " + path)


def multipart(fields, file_field, file_name, payload):
    boundary = "----nx" + uuid.uuid4().hex
    parts = []
    for name, value in fields.items():
        parts.append(("--%s\r\nContent-Disposition: form-data; name=\"%s\"\r\n\r\n%s\r\n" % (boundary, name, value)).encode("utf-8"))
    parts.append(("--%s\r\nContent-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\nContent-Type: application/octet-stream\r\n\r\n" % (boundary, file_field, file_name)).encode("utf-8"))
    parts.append(payload)
    parts.append(("\r\n--%s--\r\n" % boundary).encode("utf-8"))
    return b"".join(parts), "multipart/form-data; boundary=" + boundary


def chosen(files_dir, include, exclude):
    out = []
    for root, _, names in os.walk(files_dir):
        for name in sorted(names):
            rel = os.path.relpath(os.path.join(root, name), files_dir).replace(os.sep, "/")
            if name in NEVER:
                continue
            parts = rel.split("/")
            matches = lambda pattern: fnmatch.fnmatch(rel, pattern) or fnmatch.fnmatch(name, pattern) or pattern in parts[:-1]   # noqa: E731
            if include and not any(matches(p) for p in include):
                continue
            if exclude and any(matches(p) for p in exclude):
                continue
            out.append(rel)
    return sorted(out)


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("case_dir")
    parser.add_argument("--collection", required=True)
    parser.add_argument("--include", action="append", default=[])
    parser.add_argument("--exclude", action="append", default=[])
    parser.add_argument("--url", default="http://127.0.0.1:8091")
    parser.add_argument("--tenant", default="default")
    parser.add_argument("--timezone", default="Asia/Karachi")
    parser.add_argument("--date-order", default="DMY")
    parser.add_argument("--actor", default="reference-case-loader")
    parser.add_argument("--apply", action="store_true", help="really send the files (otherwise only list them)")
    args = parser.parse_args(argv)
    files_dir = os.path.join(args.case_dir, "files")
    plan = chosen(files_dir, args.include, args.exclude)
    total = sum(os.path.getsize(os.path.join(files_dir, *rel.split("/"))) for rel in plan)
    print("%d files, %.1f MB, to collection %s (tenant %s, time zone %s, date order %s)" % (len(plan), total / 1e6, args.collection, args.tenant, args.timezone, args.date_order))
    if not args.apply:
        for rel in plan:
            print("  would send", rel)
        print("DRY RUN: nothing was sent (add --apply)")
        return 0
    key = api_key()
    results = []
    for rel in plan:
        path = os.path.join(files_dir, *rel.split("/"))
        with open(path, "rb") as handle:
            payload = handle.read()
        fields = {"tenant_id": args.tenant, "collection_id": args.collection, "case_id": args.collection, "user_id": args.actor, "record_type": "auto",
                  "source_timezone": args.timezone, "source_date_order": args.date_order, "skip_kb_mirror": "true"}
        body, content_type = multipart(fields, "file", os.path.basename(rel), payload)
        request = urllib.request.Request(args.url.rstrip("/") + "/webhooks/records/upload", data=body, method="POST")
        for header, value in (("Content-Type", content_type), ("Authorization", "Bearer " + key), ("X-Forensic-Tenant-ID", args.tenant), ("X-Forensic-Actor-ID", args.actor),
                              ("X-Forensic-Subject-ID", args.actor), ("X-Forensic-Actor-Role", "admin"), ("X-Forensic-Collection-ID", args.collection), ("X-Forensic-Case-ID", args.collection)):
            request.add_header(header, value)
        try:
            with urllib.request.urlopen(request, timeout=600) as response:
                code, text = response.status, response.read().decode("utf-8", "replace")
        except urllib.error.HTTPError as error:
            code, text = error.code, error.read().decode("utf-8", "replace")
        try:
            reply = json.loads(text)
        except ValueError:
            reply = {"raw": text[:200]}
        shown = {k: reply.get(k) for k in ("status", "evidence_id", "job_id", "batch_id", "detected_record_type", "reason", "error", "message") if reply.get(k) is not None}
        print("%-48s HTTP %s %s" % (rel[:48], code, json.dumps(shown)[:200]))
        results.append((rel, code, reply))
    bad = [r for r in results if r[1] >= 300]
    print("%d sent, %d refused" % (len(results) - len(bad), len(bad)))
    return 0 if not bad else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
