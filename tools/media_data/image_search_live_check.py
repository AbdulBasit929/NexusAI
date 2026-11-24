#!/usr/bin/env python3
"""Live checks of image search against the running API (read-only for the case: a search writes only its own log row).

    python tools/media_data/image_search_live_check.py --collection orion-lahore-reference-20261030 words "a red car at night"
    python tools/media_data/image_search_live_check.py --collection ... file C:/path/to/photo.jpg      # reference image: exact and similar
    python tools/media_data/image_search_live_check.py --collection ... evidence <evidence_id>           # copies and similar of a stored image
    python tools/media_data/image_search_live_check.py --collection ... ask "show me photos of a person standing near a motorcycle"

The API key and the tenant are read from the stack's secrets file by name and never printed.
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

SECRETS = r"C:\Users\sheik\Workspace\Office\Projects\NexusAI\.env.forensic-runtime.local"
API = "http://127.0.0.1:8091"


def secret(name):
    for line in open(SECRETS, encoding="utf-8"):
        if line.startswith(name + "="):
            return line.split("=", 1)[1].strip()
    sys.exit(f"{name} is not in the secrets file")


def headers(collection):
    tenant = secret("FORENSIC_RECORDS_TENANT_ID") or "default"
    return tenant, {"Authorization": "Bearer " + secret("FORENSIC_RECORDS_API_KEY"), "X-Forensic-Tenant-ID": tenant, "X-Forensic-Collection-ID": collection,
                    "X-Forensic-Actor-ID": "image-search-check", "X-Forensic-Subject-ID": "image-search-check", "X-Forensic-Actor-Role": "user"}


def call(request, timeout=300):
    started = time.time()
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return response.status, json.loads(response.read().decode("utf-8")), time.time() - started, dict(response.headers)
    except urllib.error.HTTPError as error:
        return error.code, json.loads(error.read().decode("utf-8") or "{}"), time.time() - started, dict(error.headers)


def show(status, body, seconds, kind, response_headers=None):
    print(f"HTTP {status} in {seconds:.1f} s")
    if status != 200:
        print("  ", body)
        return
    if "results" in body and "coverage" in body:  # the plain endpoint answer
        print("  coverage:", body["coverage"]["statement"])
        for h in body["results"][:12]:
            print(f"   {h.get('rank')}. {h['original_filename']}  [{h['tier']}]  hash={h.get('hash_distance_bits')} cos={h.get('cosine_distance')}")
        print("  limitations:", len(body.get("limitations", [])), "| translated:", body.get("translated_query"))
        return
    answer = body.get("answer", {})
    print("  route:", body.get("route"), "| state:", (body.get("evidence") or {}).get("result_state"), "| completeness:", (body.get("evidence") or {}).get("search_completeness"))
    print("  answer:", answer.get("answer"))
    for r in (body.get("evidence") or {}).get("results", [])[:12]:
        m = r.get("metadata", {})
        print(f"   - {m.get('source_file')}  [{m.get('match_kind')}] score={round(r.get('score', 0), 3)}")
    if response_headers:
        for k in ("X-Image-Intent", "X-Text-Intent"):
            if k in response_headers:
                print("  ", k + ":", response_headers[k])


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--collection", required=True)
    parser.add_argument("mode", choices=("words", "file", "evidence", "ask"))
    parser.add_argument("value")
    parser.add_argument("--ask-format", action="store_true", help="ask the plain endpoint for an Ask-shaped answer")
    args = parser.parse_args()
    tenant, h = headers(args.collection)
    base = {"tenant_id": tenant, "collection_id": args.collection}
    if args.mode == "words":
        query = urllib.parse.urlencode({**base, "kind": "by_words", "q": args.value, "top_k": 8, **({"format": "ask"} if args.ask_format else {})})
        show(*call(urllib.request.Request(f"{API}/images/search?{query}", headers=h))[:3], "words")
    elif args.mode == "evidence":
        query = urllib.parse.urlencode({**base, "kind": "reference", "evidence_id": args.value, "top_k": 8, "format": "ask"})
        status, body, seconds, rh = call(urllib.request.Request(f"{API}/images/search?{query}", headers=h))
        show(status, body, seconds, "evidence", rh)
    elif args.mode == "file":
        boundary = uuid.uuid4().hex
        data = open(args.value, "rb").read()
        name = args.value.replace("\\", "/").rsplit("/", 1)[-1]
        body = (f"--{boundary}\r\nContent-Disposition: form-data; name=\"image\"; filename=\"{name}\"\r\nContent-Type: {mimetypes.guess_type(name)[0] or 'application/octet-stream'}\r\n\r\n").encode() + data + f"\r\n--{boundary}--\r\n".encode()
        query = urllib.parse.urlencode({**base, "kind": "reference_upload", "top_k": 8, "format": "ask"})
        request = urllib.request.Request(f"{API}/images/search?{query}", data=body, method="POST", headers={**h, "Content-Type": f"multipart/form-data; boundary={boundary}"})
        status, out, seconds, rh = call(request)
        show(status, out, seconds, "file", rh)
    else:
        payload = json.dumps({"tenant_id": tenant, "collection_id": args.collection, "query": args.value}).encode()
        request = urllib.request.Request(f"{API}/query/hybrid", data=payload, method="POST", headers={**h, "Content-Type": "application/json"})
        status, body, seconds, rh = call(request, timeout=600)
        show(status, body, seconds, "ask", rh)


if __name__ == "__main__":
    main()
