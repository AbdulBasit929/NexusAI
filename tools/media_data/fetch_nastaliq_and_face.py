#!/usr/bin/env python3
"""Fetches the public files approved on 2026-11-16 for the Urdu Nastaliq reader and the face matching measurement, pinned to the commit the Hugging Face API reports, outside the repository.

    python -I -X utf8 tools/media_data/fetch_nastaliq_and_face.py [--only qwen2vl,qaari,auraface,unb]

Ledger rows first (docs/compliance/licence-ledger.csv): MOD-045 Qwen2-VL-2B-Instruct (Apache-2.0 tag), MOD-046 the Qaari-0.1 Urdu adapter, MOD-032 AuraFace-v1 (recognition file only; the bundled SCRFD detector
is NOT fetched: it is InsightFace's and non-commercial), DAT-024 Urdu Newspaper Benchmark (no licence stated: private evaluation only). Everything goes under C:/tmp/nexusai-media-data, never into the
repository. Every file is hashed (SHA-256) and written to the manifest with its revision, size and source URL. A partial file is resumed. Public files only; no credential is used or sent.
"""
import argparse
import hashlib
import json
import os
import sys
import time
import urllib.request

ROOT = os.environ.get("NEXUSAI_MEDIA_DATA", "C:/tmp/nexusai-media-data")
MANIFEST = os.path.join(ROOT, "manifest-20261116-nastaliq-face.jsonl")
HEADERS = {"User-Agent": "nexusai-media-fetch/1"}
TARGETS = {
    "qwen2vl": {"kind": "models", "repo": "Qwen/Qwen2-VL-2B-Instruct", "dest": "models/ocr/qwen2-vl-2b-instruct", "ledger": "MOD-045", "only": None},
    "qaari": {"kind": "models", "repo": "oddadmix/Qaari-0.1-Urdu-OCR-VL-2B-Instruct", "dest": "models/ocr/qaari-0.1-urdu-adapter", "ledger": "MOD-046", "only": None},
    "auraface": {"kind": "models", "repo": "fal/AuraFace-v1", "dest": "models/face/auraface-v1", "ledger": "MOD-032", "only": ("glintr100.onnx", "LICENSE.md", "README.md")},
    "unb": {"kind": "datasets", "repo": "ULRs/Urdu-Newspaper-Benchmark", "dest": "datasets/urdu-newspaper-benchmark", "ledger": "DAT-024", "only": None},
}


def get_json(url):
    with urllib.request.urlopen(urllib.request.Request(url, headers=HEADERS), timeout=60) as response:
        return json.loads(response.read().decode("utf-8"))


def download(url, path):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    have = os.path.getsize(path) if os.path.exists(path) else 0
    request = urllib.request.Request(url, headers=dict(HEADERS, **({"Range": "bytes=%d-" % have} if have else {})))
    try:
        response = urllib.request.urlopen(request, timeout=120)
    except urllib.error.HTTPError as error:
        if error.code == 416:       # already complete
            return
        raise
    mode = "ab" if have and response.status == 206 else "wb"
    started = time.time()
    done = have if mode == "ab" else 0
    with response, open(path, mode) as handle:
        while True:
            block = response.read(1 << 20)
            if not block:
                break
            handle.write(block)
            done += len(block)
    print("  %-60s %8.1f MB in %4.0f s" % (os.path.basename(path), done / 1e6, time.time() - started), flush=True)


def sha256(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for block in iter(lambda: handle.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


def fetch(name):
    t = TARGETS[name]
    info = get_json("https://huggingface.co/api/%s/%s" % (t["kind"], t["repo"]))
    revision = info["sha"]
    files = [s["rfilename"] for s in info["siblings"] if not t["only"] or s["rfilename"] in t["only"]]
    prefix = "" if t["kind"] == "models" else "datasets/"
    print("%s: %s @ %s (%d files)" % (name, t["repo"], revision[:12], len(files)), flush=True)
    for rel in files:
        if rel == ".gitattributes":
            continue
        url = "https://huggingface.co/%s%s/resolve/%s/%s" % (prefix, t["repo"], revision, rel)
        path = os.path.join(ROOT, t["dest"], revision[:12], *rel.split("/"))
        download(url, path)
        record = {"ledger": t["ledger"], "repo": t["repo"], "revision": revision, "file": rel, "bytes": os.path.getsize(path), "sha256": sha256(path), "url": url, "path": path.replace("\\", "/"), "fetched": time.strftime("%Y-%m-%d %H:%M:%S")}
        with open(MANIFEST, "a", encoding="utf-8", newline="\n") as handle:
            handle.write(json.dumps(record, ensure_ascii=False) + "\n")


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--only", default=",".join(TARGETS))
    args = ap.parse_args(argv)
    for name in args.only.split(","):
        fetch(name.strip())
    print("done; manifest:", MANIFEST)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
