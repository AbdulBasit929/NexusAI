#!/usr/bin/env python3
"""Fetches the three EasyOCR model files approved on 2026-11-17 (ledger MOD-047 to MOD-049) from the JaidedAI GitHub releases, outside the repository, with SHA-256 and size in the manifest.

    python -I -X utf8 tools/media_data/fetch_easyocr.py

Files land in <data>/models/ocr/easyocr/ and the weights are unpacked into <data>/models/ocr/easyocr/model/ (the folder EasyOCR reads with model_storage_directory). Public files only; no login.
"""
import hashlib
import json
import os
import sys
import time
import urllib.request
import zipfile

ROOT = os.environ.get("NEXUSAI_MEDIA_DATA", "C:/tmp/nexusai-media-data")
DEST = os.path.join(ROOT, "models/ocr/easyocr")
MANIFEST = os.path.join(ROOT, "manifest-20261117-easyocr.jsonl")
FILES = [
    ("MOD-047", "craft_mlt_25k.zip", "https://github.com/JaidedAI/EasyOCR/releases/download/pre-v1.1.6/craft_mlt_25k.zip", 77251756),
    ("MOD-048", "arabic.zip", "https://github.com/JaidedAI/EasyOCR/releases/download/pre-v1.1.6/arabic.zip", 199916962),
    ("MOD-049", "english_g2.zip", "https://github.com/JaidedAI/EasyOCR/releases/download/v1.3/english_g2.zip", 14040947),
]


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for block in iter(lambda: f.read(1 << 20), b""):
            h.update(block)
    return h.hexdigest()


PARTS = 8


def final_url(url):
    request = urllib.request.Request(url, method="HEAD", headers={"User-Agent": "nexusai-media-fetch/1"})
    with urllib.request.urlopen(request, timeout=60) as r:
        return r.geturl()


def download_parallel(url, path, size):
    """GitHub release assets are throttled per connection (about 50 KB/s here): the file is fetched in PARTS ranges at once and joined. A failed range is retried three times."""
    import concurrent.futures
    target = final_url(url)
    step = -(-size // PARTS)
    ranges = [(i * step, min(size, (i + 1) * step) - 1) for i in range(PARTS)]

    def fetch(index):
        a, b = ranges[index]
        part = "%s.part%d" % (path, index)
        for attempt in range(3):
            have = os.path.getsize(part) if os.path.exists(part) else 0
            if have >= b - a + 1:
                return part
            request = urllib.request.Request(target, headers={"User-Agent": "nexusai-media-fetch/1", "Range": "bytes=%d-%d" % (a + have, b)})
            try:
                with urllib.request.urlopen(request, timeout=120) as r, open(part, "ab") as f:
                    while True:
                        block = r.read(1 << 18)
                        if not block:
                            break
                        f.write(block)
            except Exception:
                continue
        if os.path.getsize(part) != b - a + 1:
            raise SystemExit("range %d of %s incomplete" % (index, path))
        return part
    with concurrent.futures.ThreadPoolExecutor(PARTS) as pool:
        parts = list(pool.map(fetch, range(PARTS)))
    with open(path, "wb") as out:
        for part in parts:
            out.write(open(part, "rb").read())
            os.remove(part)


def main():
    os.makedirs(os.path.join(DEST, "model"), exist_ok=True)
    for ledger, name, url, expected in FILES:
        path = os.path.join(DEST, name)
        if not (os.path.exists(path) and os.path.getsize(path) == expected):
            started = time.time()
            download_parallel(url, path, expected)
            print("  %-22s %7.1f MB in %4.0f s" % (name, os.path.getsize(path) / 1e6, time.time() - started), flush=True)
        if os.path.getsize(path) != expected:
            raise SystemExit("%s: %d bytes, expected %d" % (name, os.path.getsize(path), expected))
        with zipfile.ZipFile(path) as z:
            members = z.namelist()
            z.extractall(os.path.join(DEST, "model"))
        record = {"ledger": ledger, "file": name, "bytes": os.path.getsize(path), "sha256": sha256(path), "url": url, "members": members, "fetched": time.strftime("%Y-%m-%d %H:%M:%S")}
        open(MANIFEST, "a", encoding="utf-8", newline="\n").write(json.dumps(record) + "\n")
        print("  ok", name, members, record["sha256"][:16], flush=True)
    print("done; manifest:", MANIFEST)


if __name__ == "__main__":
    sys.exit(main())
