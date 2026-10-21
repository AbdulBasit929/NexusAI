#!/usr/bin/env python3
"""Provision the worker's read-only model folder (/models/media) from the fetched, hash-checked files, outside any git tree's tracked files (models/ is git-ignored).

    python -X utf8 tools/media_data/provision_models.py            # build models/media in this repository checkout and print the environment pins

What it does: copies the existing assets the worker already uses (plate detector and reader, SigLIP, the Paddle model directories) from the old checkout's models/media, then adds the new assets from
C:/tmp/nexusai-media-data (Tesseract best and fast language data with the repository revision they came from, the PP-OCRv5 ONNX detector and Latin recogniser), verifying every file against the
fetch manifest first. It prints the SHA-256 values the worker's environment must pin (the worker refuses a model whose hash differs) and writes models/media/PROVENANCE.json. Nothing is
downloaded here and nothing outside models/media is changed.
"""
import hashlib
import json
import os
import shutil
import sys
import time

REPO = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "C:/tmp/nexusai-media-data")
OLD = os.path.abspath(os.path.join(REPO, "..", "NexusAI", "models", "media"))
DEST = os.path.join(REPO, "models", "media")
TESSDATA_BEST_REV = "e12c65a915945e4c28e237a9b52bc4a8f39a0cec"  # tesseract-ocr/tessdata_best main, read from the GitHub API on 2026-10-21 (Apache-2.0 LICENSE opened)
TESSDATA_FAST_REV = "65727574dfcd264acbb0c3e07860e4e9e9b22185"  # the revision the compose file already names; the two pinned hashes equal the files fetched from main on 2026-10-21


def sha256(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for block in iter(lambda: handle.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


def manifest():
    out = {}
    for line in open(os.path.join(DATA, "manifest.jsonl"), encoding="utf-8"):
        if line.strip():
            item = json.loads(line)
            out[item["rel"]] = item
    return out


def copy(src, dst):
    os.makedirs(os.path.dirname(dst), exist_ok=True)
    if os.path.exists(dst) and os.path.getsize(dst) == os.path.getsize(src):
        return
    shutil.copyfile(src, dst)


def main():
    if not os.path.isdir(OLD):
        sys.exit(f"the existing model folder is missing: {OLD}")
    os.makedirs(DEST, exist_ok=True)
    print(f"copying the existing assets from {OLD} ...")
    for name in os.listdir(OLD):
        source, target = os.path.join(OLD, name), os.path.join(DEST, name)
        if os.path.isdir(source):
            shutil.copytree(source, target, dirs_exist_ok=True,
                            copy_function=lambda s, d: None if os.path.exists(d) and os.path.getsize(d) == os.path.getsize(s) else shutil.copy2(s, d))  # a file already there with the same size is left alone
    known = manifest()
    pins, provenance = {}, []

    def add(rel, dest_rel, note, licence):
        entry = known.get(rel)
        source = os.path.join(DATA, rel)
        if entry is None or not os.path.exists(source):
            sys.exit(f"not fetched: {rel}")
        if sha256(source) != entry["sha256"]:
            sys.exit(f"hash mismatch against the fetch manifest: {rel}")
        target = os.path.join(DEST, dest_rel)
        copy(source, target)
        provenance.append({"file": dest_rel.replace(os.sep, "/"), "sha256": entry["sha256"], "bytes": entry["bytes"], "source": entry["url"], "licence": licence, "note": note})
        return entry["sha256"]

    for language in ("eng", "urd", "osd"):
        # the orientation and script data (osd) is the same file in both repositories; the fast copy was fetched
        best_source = f"tessdata/best/{language}.traineddata" if language != "osd" else "tessdata/fast/osd.traineddata"
        pins[f"tessdata-best-{language}"] = add(best_source, f"nexusai/models/tessdata-best/{TESSDATA_BEST_REV}/{language}.traineddata", f"tessdata_best at {TESSDATA_BEST_REV}" + (" (osd from the fast fetch)" if language == "osd" else ""), "Apache-2.0")
        pins[f"tessdata-fast-{language}"] = add(f"tessdata/fast/{language}.traineddata", f"nexusai/models/tessdata-fast/{TESSDATA_FAST_REV}/{language}.traineddata", "tessdata_fast (hash equals the pinned value in the compose file)", "Apache-2.0")
    pins["onnx-det"] = add("models/ocr/PP-OCRv5_mobile_det_onnx/inference.onnx", "onnx-ocr/PP-OCRv5_mobile_det/inference.onnx", "PaddlePaddle/PP-OCRv5_mobile_det_onnx", "Apache-2.0 (hub tag)")
    add("models/ocr/PP-OCRv5_mobile_det_onnx/inference.yml", "onnx-ocr/PP-OCRv5_mobile_det/inference.yml", "detector configuration", "Apache-2.0 (hub tag)")
    pins["onnx-latin"] = add("models/ocr/latin_PP-OCRv5_mobile_rec_onnx/inference.onnx", "onnx-ocr/latin_PP-OCRv5_mobile_rec/inference.onnx", "PaddlePaddle/latin_PP-OCRv5_mobile_rec_onnx", "Apache-2.0 (hub tag)")
    add("models/ocr/latin_PP-OCRv5_mobile_rec_onnx/inference.yml", "onnx-ocr/latin_PP-OCRv5_mobile_rec/inference.yml", "recogniser configuration and character dictionary", "Apache-2.0 (hub tag)")
    pins["onnx-script-probe"] = add("models/ocr/arabic_PP-OCRv5_mobile_rec_onnx/inference.onnx", "onnx-ocr/arabic_PP-OCRv5_mobile_rec/inference.onnx",
                                    "PaddlePaddle/arabic_PP-OCRv5_mobile_rec_onnx, used ONLY to tell Arabic-script pages from Latin ones (not read as OCR output)", "Apache-2.0 (hub tag)")
    add("models/ocr/arabic_PP-OCRv5_mobile_rec_onnx/inference.yml", "onnx-ocr/arabic_PP-OCRv5_mobile_rec/inference.yml", "script probe configuration and character dictionary", "Apache-2.0 (hub tag)")
    with open(os.path.join(DEST, "PROVENANCE.json"), "w", encoding="utf-8") as handle:
        json.dump({"written": time.strftime("%Y-%m-%dT%H:%M:%S"), "files": provenance}, handle, indent=1)
    print("\nenvironment pins (set in docker-compose.forensic-records.media.yaml):")
    for key, value in pins.items():
        print(f"  {key:22} {value}")
    print(f"\nmodel folder ready: {DEST}  (mount it with NEXUSAI_MEDIA_MODEL_DIR)")


if __name__ == "__main__":
    main()
