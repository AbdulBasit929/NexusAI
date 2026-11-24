#!/usr/bin/env python3
"""Fetch the image-retrieval models of the bake-off (docs/data/face-image-search-preregistration-20261119.md, protocol V) and write a hash manifest. Needs the network, so it runs in a throw-away container with the
data folder mounted read-write and NOTHING else:

    docker run --rm --memory 1500m -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro --entrypoint python nexusai-media-eval-face:20261119 /repo/tools/media_data/fetch_retrieval_models_20261119.py

Only safetensors and the small config/tokenizer files are taken (no pickled .bin checkpoints).
"""
import hashlib
import json
import os
import time
from pathlib import Path

from huggingface_hub import HfApi, snapshot_download

DATA = Path("/data")
MODELS = {
    "image/siglip2-base-patch16-224": "google/siglip2-base-patch16-224",
    "image/dinov2-base": "facebook/dinov2-base",
    "text/opus-mt-en-ur": "Helsinki-NLP/opus-mt-en-ur",
}
PATTERNS = ["*.json", "*.safetensors", "*.model", "*.txt", "*.spm", "source.spm", "target.spm", "vocab*", "LICENSE*", "README.md"]


def sha256(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for block in iter(lambda: handle.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


def main():
    api = HfApi()
    manifest = DATA / "manifest-20261119-faces.jsonl"
    for folder, repo in MODELS.items():
        info = api.model_info(repo)
        target = DATA / "models" / folder
        if (target / "config.json").exists():
            print("have", folder)
        else:
            snapshot_download(repo, local_dir=str(target), allow_patterns=PATTERNS, revision=info.sha)
        files = sorted(p for p in target.rglob("*") if p.is_file() and ".cache" not in p.parts)
        with open(manifest, "a", encoding="utf-8") as out:
            for p in files:
                rec = {"id": f"models/{folder}/{p.relative_to(target).as_posix()}", "url": f"https://huggingface.co/{repo}", "revision": info.sha, "bytes": p.stat().st_size, "sha256": sha256(p),
                       "note": f"licence tag {getattr(info, 'card_data', None) and info.card_data.get('license')}", "fetched": time.strftime("%Y-%m-%d")}
                out.write(json.dumps(rec) + "\n")
        print(folder, repo, info.sha, [p.name for p in files])


if __name__ == "__main__":
    main()
