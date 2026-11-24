#!/usr/bin/env python3
"""Runs every reference file of the image-search test pack through the live reference search (POST /images/search, kind reference_upload) and writes what came back, so that the manual test guide can quote
real, measured results instead of promises. Read-only for the case (a search writes only its own log row). Needs no language model.

    python tools/media_data/image_search_reference_run.py --collection orion-lahore-reference-20261030 --pack C:/NexusAI-Showcase/orion/image-search-test-pack

Writes <pack>/measured-results.json and prints one line per file. The API key and tenant are read from the stack's secrets file by name and never printed.
"""
import argparse
import json
import mimetypes
import sys
import time
import urllib.error
import urllib.parse
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


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--collection", required=True)
    parser.add_argument("--pack", required=True)
    parser.add_argument("--top-k", type=int, default=8)
    args = parser.parse_args()
    pack = Path(args.pack)
    manifest = json.load(open(pack / "manifest.json"))
    tenant = secret("FORENSIC_RECORDS_TENANT_ID") or "default"
    base_headers = {"Authorization": "Bearer " + secret("FORENSIC_RECORDS_API_KEY"), "X-Forensic-Tenant-ID": tenant, "X-Forensic-Collection-ID": args.collection,
                    "X-Forensic-Actor-ID": "image-search-pack-run", "X-Forensic-Subject-ID": "image-search-pack-run", "X-Forensic-Actor-Role": "user"}
    rows = []
    for item in manifest["files"]:
        if not item["file"].startswith("reference/"):
            continue
        path = pack / item["file"]
        boundary = uuid.uuid4().hex
        body = (f"--{boundary}\r\nContent-Disposition: form-data; name=\"image\"; filename=\"{path.name}\"\r\nContent-Type: {mimetypes.guess_type(path.name)[0] or 'application/octet-stream'}\r\n\r\n").encode() \
            + path.read_bytes() + f"\r\n--{boundary}--\r\n".encode()
        query = urllib.parse.urlencode({"tenant_id": tenant, "collection_id": args.collection, "kind": "reference_upload", "top_k": args.top_k})
        request = urllib.request.Request(f"{API}/images/search?{query}", data=body, method="POST", headers={**base_headers, "Content-Type": f"multipart/form-data; boundary={boundary}"})
        started = time.time()
        try:
            with urllib.request.urlopen(request, timeout=300) as response:
                answer = json.loads(response.read().decode("utf-8"))
            status = 200
        except urllib.error.HTTPError as error:
            answer, status = {"error": error.read().decode("utf-8")[:200]}, error.code
        seconds = time.time() - started
        hits = [{"file": h["original_filename"], "tier": h["tier"], "hash_bits": h.get("hash_distance_bits"), "cosine": h.get("cosine_distance"), "similarity": h.get("similarity")} for h in answer.get("results", [])]
        copies = [h for h in hits if h["tier"] in ("identical_file", "same_image", "same_photograph")]
        row = {"file": item["file"], "purpose": item.get("purpose"), "base": item.get("base"), "http": status, "seconds": round(seconds, 1), "copies": copies, "other": [h for h in hits if h not in copies][:3]}
        rows.append(row)
        found = ", ".join(f"{h['file']} [{h['tier']}]" for h in copies) or "no copy"
        print(f"{item['file'].split('/')[-1]:<44} {found}", flush=True)
    json.dump({"collection": args.collection, "at": time.strftime("%Y-%m-%d %H:%M:%S"), "rows": rows}, open(pack / "measured-results.json", "w", encoding="utf-8"), indent=1)
    return 0


if __name__ == "__main__":
    sys.exit(main())
