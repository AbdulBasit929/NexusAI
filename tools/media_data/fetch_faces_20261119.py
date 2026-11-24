#!/usr/bin/env python3
"""Fetch the public models and datasets of the face and image-search bake-off (docs/data/face-image-search-preregistration-20261119.md).

Runs on the host (it needs the network); every file lands in its own folder under the media data root and its SHA-256 is written to the manifest, so a later run can prove it read the same bytes.
Nothing is executed or unpacked here: archives are only downloaded (they are unpacked later inside a container without network).

    python tools/media_data/fetch_faces_20261119.py --only sface blazeface
    python tools/media_data/fetch_faces_20261119.py --list
"""
import argparse
import hashlib
import json
import os
import sys
import time
import urllib.request
from pathlib import Path

ROOT = Path(os.environ.get("NEXUSAI_MEDIA_DATA_HOST", "C:/tmp/nexusai-media-data"))
MANIFEST = ROOT / "manifest-20261119-faces.jsonl"

HF = "https://huggingface.co"
ITEMS = {
    "sface": [
        ("models/face/sface/face_recognition_sface_2021dec.onnx", f"{HF}/opencv/face_recognition_sface/resolve/main/face_recognition_sface_2021dec.onnx", "SFace recogniser (OpenCV Zoo), Apache-2.0 (LICENSE read)"),
        ("models/face/sface/LICENSE", f"{HF}/opencv/face_recognition_sface/resolve/main/LICENSE", "SFace licence text"),
        ("models/face/sface/README.md", f"{HF}/opencv/face_recognition_sface/resolve/main/README.md", "SFace README"),
    ],
    "blazeface": [
        ("models/face/blazeface/blaze_face_short_range.tflite", "https://storage.googleapis.com/mediapipe-models/face_detector/blaze_face_short_range/float16/1/blaze_face_short_range.tflite", "BlazeFace short range (MediaPipe), Apache-2.0 per model card"),
        ("models/face/blazeface/model-card.pdf", "https://storage.googleapis.com/mediapipe-assets/MediaPipe%20BlazeFace%20Model%20Card%20(Short%20Range).pdf", "BlazeFace model card"),
    ],
    "verification": [
        (f"datasets/face/verification/{name}.bin", f"{HF}/datasets/gaunernst/face-recognition-eval/resolve/main/{name}.bin", f"verification pairs {name} (unofficial mirror, pickle: restricted reader only)")
        for name in ("lfw", "cfp_fp", "agedb_30", "calfw", "cplfw")
    ],
    "fairface": [
        ("datasets/face/fairface/validation-1.25.parquet", f"{HF}/datasets/HuggingFaceM4/FairFace/resolve/main/1.25/validation-00000-of-00001-09e3e67bb00ab4ec.parquet", "FairFace validation, padding 1.25, CC BY 4.0"),
        ("datasets/face/fairface/README.md", f"{HF}/datasets/HuggingFaceM4/FairFace/resolve/main/README.md", "FairFace card"),
    ],
    "widerface": [
        ("datasets/face/widerface/WIDER_val.zip", f"{HF}/datasets/CUHK-CSE/wider_face/resolve/main/data/WIDER_val.zip", "WIDER FACE validation images, cc-by-nc-nd-4.0, evaluation only"),
        ("datasets/face/widerface/wider_face_split.zip", f"{HF}/datasets/CUHK-CSE/wider_face/resolve/main/data/wider_face_split.zip", "WIDER FACE annotations"),
    ],
    "flickr1k": [
        ("datasets/retrieval/flickr1k/images_flickr_1k_test.zip", f"{HF}/datasets/nlphuji/flickr_1k_test_image_text_retrieval/resolve/main/images_flickr_1k_test.zip", "Flickr30k 1K test images (non-commercial research)"),
        ("datasets/retrieval/flickr1k/test_1k_flickr.csv", f"{HF}/datasets/nlphuji/flickr_1k_test_image_text_retrieval/resolve/main/test_1k_flickr.csv", "Flickr30k 1K test captions"),
    ],
    "opus-weights": [
        ("models/text/opus-mt-en-ur/pytorch_model.bin", f"{HF}/Helsinki-NLP/opus-mt-en-ur/resolve/main/pytorch_model.bin", "opus-mt-en-ur weights (Apache-2.0 per hub metadata); a pickled checkpoint: loaded only with torch weights_only inside a container without network"),
    ],
    "lfw": [
        ("datasets/face/lfw-orig/lfw.tgz", "https://ndownloader.figshare.com/files/5976018", "LFW original images (the scikit-learn project's figshare mirror of the UMass archive; the UMass host did not answer on 2026-11-19)"),
        ("datasets/face/lfw-orig/pairs.txt", "https://ndownloader.figshare.com/files/5976006", "LFW standard pairs (10 folds), same mirror"),
    ],
    "ofiq-images": [
        ("datasets/face/ofiq/OFIQ-IMAGES.zip", "https://standards.iso.org/iso-iec/29794/-5/ed-1/en/OFIQ-IMAGES.zip", "OFIQ conformance test images (ISO portal); their licence file is read after download"),
        ("datasets/face/ofiq/expected_results.csv", "https://raw.githubusercontent.com/BSI-OFIQ/OFIQ-Project/main/data/tests/expected_results/expected_results.csv", "OFIQ C++ expected results for the conformance images (MIT repository)"),
    ],
    "ofiq": [
        ("models/face/ofiq/OFIQ-MODELS.zip", "https://standards.iso.org/iso-iec/29794/-5/ed-1/en/OFIQ-MODELS.zip", "OFIQ model files (ISO portal); licence files inside are read after download"),
    ],
}


def sha256_of(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for block in iter(lambda: handle.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


def download(url, dest, attempts=60):
    """Download with resume: some servers (the ISO portal) cut a long transfer; the next attempt continues from the bytes already on disk with an HTTP Range request."""
    dest.parent.mkdir(parents=True, exist_ok=True)
    partial = dest.with_suffix(dest.suffix + ".part")
    total = 0
    for attempt in range(1, attempts + 1):
        have = partial.stat().st_size if partial.exists() else 0
        headers = {"User-Agent": "nexusai-evaluation-fetch/1"}
        if have:
            headers["Range"] = f"bytes={have}-"
        try:
            with urllib.request.urlopen(urllib.request.Request(url, headers=headers), timeout=120) as response:
                status = response.status
                length = int(response.headers.get("Content-Length") or 0)
                if status == 206:
                    total = have + length
                    mode = "ab"
                else:  # the server ignored the range: start again
                    total = length
                    mode, have = "wb", 0
                with open(partial, mode) as out:
                    while True:
                        block = response.read(1 << 20)
                        if not block:
                            break
                        out.write(block)
        except Exception as error:
            print(f"  attempt {attempt}: {error}", flush=True)
        size = partial.stat().st_size if partial.exists() else 0
        if total and size == total:
            partial.replace(dest)
            return
        if total and size > total:
            raise RuntimeError(f"too many bytes: {size} of {total}")
        time.sleep(3)
    raise RuntimeError(f"gave up after {attempts} attempts: {partial.stat().st_size if partial.exists() else 0} of {total} bytes")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--only", nargs="*", default=[])
    parser.add_argument("--list", action="store_true")
    args = parser.parse_args()
    if args.list:
        for group, files in ITEMS.items():
            print(group, [f[0] for f in files])
        return 0
    groups = args.only or list(ITEMS)
    unknown = [g for g in groups if g not in ITEMS]
    if unknown:
        print("unknown group(s):", unknown)
        return 2
    problems = 0
    for group in groups:
        for rel, url, note in ITEMS[group]:
            dest = ROOT / rel
            if dest.exists():
                print(f"have  {rel} ({dest.stat().st_size} bytes)")
            else:
                started = time.time()
                try:
                    download(url, dest)
                except Exception as error:  # a failed fetch is reported, never hidden
                    print(f"FAIL  {rel}: {error}")
                    problems += 1
                    continue
                print(f"fetched {rel} ({dest.stat().st_size} bytes, {time.time() - started:.0f} s)")
            digest = sha256_of(dest)
            record = {"id": rel, "url": url, "bytes": dest.stat().st_size, "sha256": digest, "note": note, "fetched": time.strftime("%Y-%m-%d")}
            known = set()
            if MANIFEST.exists():
                known = {json.loads(line)["id"] + json.loads(line)["sha256"] for line in open(MANIFEST, encoding="utf-8") if line.strip()}
            if rel + digest not in known:
                with open(MANIFEST, "a", encoding="utf-8") as out:
                    out.write(json.dumps(record) + "\n")
            print(f"      sha256 {digest}")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
