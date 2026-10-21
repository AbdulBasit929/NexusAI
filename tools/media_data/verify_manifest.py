#!/usr/bin/env python3
"""Check every fetched file against the manifest (size and SHA-256) and, for Hugging Face files, against the size the hub announces (x-linked-size), so a truncated download cannot be used.

    python -X utf8 tools/media_data/verify_manifest.py          # sizes against the manifest and the hub
    python -X utf8 tools/media_data/verify_manifest.py --hash   # also recompute SHA-256 (slow for the large files)
"""
import argparse
import hashlib
import json
import os
import sys
import urllib.request

ROOT = os.environ.get("NEXUSAI_MEDIA_DATA", "C:/tmp/nexusai-media-data")


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        return None


def announced_size(url):
    opener = urllib.request.build_opener(NoRedirect)
    request = urllib.request.Request(url, method="HEAD", headers={"User-Agent": "nexusai-media-verify/1"})
    try:
        response = opener.open(request, timeout=30)
        headers = response.headers
    except urllib.error.HTTPError as error:
        headers = error.headers
    value = headers.get("x-linked-size")  # only large (LFS) files announce their size; for small files the head request answers with the redirect page
    return int(value) if value else None


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--hash", action="store_true")
    args = parser.parse_args()
    bad = 0
    seen = set()
    for line in open(os.path.join(ROOT, "manifest.jsonl"), encoding="utf-8"):
        if not line.strip():
            continue
        item = json.loads(line)
        if item["rel"] in seen or item["url"].startswith("storage.googleapis.com"):
            continue
        seen.add(item["rel"])
        path = os.path.join(ROOT, item["rel"])
        if not os.path.exists(path):
            print(f"MISSING  {item['rel']}")
            bad += 1
            continue
        size = os.path.getsize(path)
        problems = []
        if size != item["bytes"]:
            problems.append(f"size {size} != manifest {item['bytes']}")
        if "huggingface.co" in item["url"]:
            hub = announced_size(item["url"])
            if hub and hub != size:
                problems.append(f"size {size} != hub {hub}")
        if args.hash:
            digest = hashlib.sha256()
            with open(path, "rb") as handle:
                for block in iter(lambda: handle.read(1 << 20), b""):
                    digest.update(block)
            if digest.hexdigest() != item["sha256"]:
                problems.append("sha256 differs from the manifest")
        print(("BAD      " if problems else "ok       ") + item["rel"] + ("  " + "; ".join(problems) if problems else ""))
        bad += 1 if problems else 0
    print(f"{len(seen)} files checked, {bad} problem(s)")
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
