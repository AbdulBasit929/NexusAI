#!/usr/bin/env python3
"""Adds the in-case files of the image-search test pack to a case through the product's own upload route (POST /webhooks/records/upload, the one the workspace uses): authenticated, retained
content-addressed, queued; the worker then reads each image (OCR, plates, and the image-search descriptors). Evidence is only ever ADDED. A file whose bytes the case already holds is not added twice.

    python tools/media_data/upload_case_images.py --collection orion-lahore-reference-20261030 --pack C:/NexusAI-Showcase/orion/image-search-test-pack [--limit 3] [--dry]

The API key and the tenant are read from the stack's secrets file by name and never printed.
"""
import argparse
import json
import mimetypes
import sys
import time
import urllib.error
import urllib.request
import uuid
from pathlib import Path

SECRETS = r"C:\Users\sheik\Workspace\Office\Projects\NexusAI\.env.forensic-runtime.local"
API = "http://127.0.0.1:8091"


def secret(name):
    for line in open(SECRETS, encoding="utf-8"):
        if line.startswith(name + "="):
            return line.split("=", 1)[1].strip()
    sys.exit(f"{name} is not in the secrets file")


def multipart(fields, file_field, path):
    boundary = uuid.uuid4().hex
    parts = []
    for name, value in fields.items():
        parts.append(f'--{boundary}\r\nContent-Disposition: form-data; name="{name}"\r\n\r\n{value}\r\n'.encode())
    data = Path(path).read_bytes()
    kind = mimetypes.guess_type(path)[0] or "application/octet-stream"
    parts.append(f'--{boundary}\r\nContent-Disposition: form-data; name="{file_field}"; filename="{Path(path).name}"\r\nContent-Type: {kind}\r\n\r\n'.encode() + data + b"\r\n")
    parts.append(f"--{boundary}--\r\n".encode())
    return b"".join(parts), boundary


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--collection", required=True)
    parser.add_argument("--pack", required=True)
    parser.add_argument("--limit", type=int, default=0)
    parser.add_argument("--dry", action="store_true")
    args = parser.parse_args()
    pack = Path(args.pack)
    manifest = json.load(open(pack / "manifest.json"))
    files = [pack / item["file"] for item in manifest["files"] if item["file"].startswith("in-case/")]
    if args.limit:
        files = files[: args.limit]
    key, tenant = secret("FORENSIC_RECORDS_API_KEY"), secret("FORENSIC_RECORDS_TENANT_ID") or "default"
    print(f"{len(files)} files for {args.collection}")
    if args.dry:
        for f in files:
            print("  would upload", f.name)
        return 0
    ok = failed = 0
    uploaded = []
    for n, path in enumerate(files, start=1):
        body, boundary = multipart({"tenant_id": tenant, "collection_id": args.collection, "case_id": args.collection, "user_id": "image-search-test-pack", "source_entry": path.name}, "file", path)
        request = urllib.request.Request(f"{API}/webhooks/records/upload", data=body, method="POST", headers={
            "Authorization": f"Bearer {key}", "Content-Type": f"multipart/form-data; boundary={boundary}", "X-Forensic-Tenant-ID": tenant, "X-Forensic-Collection-ID": args.collection,
            "X-Forensic-Case-ID": args.collection, "X-Forensic-Actor-ID": "image-search-test-pack", "X-Forensic-Subject-ID": "image-search-test-pack", "X-Forensic-Actor-Role": "user"})
        try:
            with urllib.request.urlopen(request, timeout=120) as response:
                answer = json.loads(response.read().decode() or "{}")
            ok += 1
            uploaded.append({"file": path.name, "evidence_id": answer.get("evidence_id"), "status": answer.get("status")})
            print(f"  [{n}/{len(files)}] {path.name}: accepted ({answer.get('status', '')})", flush=True)
        except urllib.error.HTTPError as error:
            failed += 1
            print(f"  [{n}/{len(files)}] {path.name}: HTTP {error.code} {error.read().decode()[:140]}", flush=True)
        except OSError as error:
            failed += 1
            print(f"  [{n}/{len(files)}] {path.name}: {error}", flush=True)
        time.sleep(0.2)
    json.dump({"collection": args.collection, "uploaded": uploaded, "at": time.strftime("%Y-%m-%d %H:%M:%S")}, open(pack / "uploaded.json", "w"), indent=1)
    print(f"done: {ok} accepted, {failed} refused")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
