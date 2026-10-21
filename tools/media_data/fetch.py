#!/usr/bin/env python3
"""Fetch the public, licensed data and model files of the media activation programme (docs/data/media-activation-programme-20261021.md) into C:/tmp/nexusai-media-data/.

    python -X utf8 tools/media_data/fetch.py --list
    python -X utf8 tools/media_data/fetch.py ocr           # one group
    python -X utf8 tools/media_data/fetch.py ocr audio     # several

Rules it enforces: only files named here (each group states its licence and the ledger row), written outside the repository, never overwritten if the size matches, every file recorded in
manifest.jsonl with its SHA-256, size, source URL, licence, ledger id and fetch time. A file that exists with a different recorded SHA-256 is refused (the manifest is the truth, nothing is silently replaced).
Resumable: an interrupted download continues from the part file. Standard library only.
"""
import argparse
import hashlib
import json
import os
import sys
import time
import urllib.request

ROOT = os.environ.get("NEXUSAI_MEDIA_DATA", "C:/tmp/nexusai-media-data")
HF = "https://huggingface.co"
GH = "https://raw.githubusercontent.com"
UA = {"User-Agent": "nexusai-media-fetch/1"}


def hf(kind, repo, path):
    prefix = "datasets/" if kind == "datasets" else ""
    return f"{HF}/{prefix}{repo}/resolve/main/{path}"


# group -> list of dict(url, rel, licence, ledger, note)
GROUPS = {
    "ocr": [
        dict(url=hf("datasets", "PuristanLabs1/urdu-ocr-1M", "nastaliq/val-00000-of-00001.parquet"), rel="urdu_ocr/nastaliq-val.parquet", licence="MIT (hub tag)", ledger="DAT-021", note="synthetic Nastaliq lines, validation split"),
        dict(url=hf("datasets", "PuristanLabs1/urdu-ocr-1M", "data/val-00000-of-00001.parquet"), rel="urdu_ocr/naskh-val.parquet", licence="MIT (hub tag)", ledger="DAT-021", note="synthetic Naskh lines, validation split"),
        dict(url=hf("datasets", "naver-clova-ix/cord-v2", "data/test-00000-of-00001-9c204eb3f4e11791.parquet"), rel="cord/test.parquet", licence="CC BY 4.0 (hub tag)", ledger="DAT-020", note="real receipt scans with text, test split"),
        dict(url=f"{GH}/tesseract-ocr/tessdata_fast/main/eng.traineddata", rel="tessdata/fast/eng.traineddata", licence="Apache-2.0", ledger="MOD-042", note="Tesseract LSTM fast, English"),
        dict(url=f"{GH}/tesseract-ocr/tessdata_fast/main/urd.traineddata", rel="tessdata/fast/urd.traineddata", licence="Apache-2.0", ledger="MOD-042", note="Tesseract LSTM fast, Urdu"),
        dict(url=f"{GH}/tesseract-ocr/tessdata_fast/main/osd.traineddata", rel="tessdata/fast/osd.traineddata", licence="Apache-2.0", ledger="MOD-042", note="orientation and script detection"),
        dict(url=f"{GH}/tesseract-ocr/tessdata_best/main/eng.traineddata", rel="tessdata/best/eng.traineddata", licence="Apache-2.0", ledger="MOD-042", note="Tesseract LSTM best, English"),
        dict(url=f"{GH}/tesseract-ocr/tessdata_best/main/urd.traineddata", rel="tessdata/best/urd.traineddata", licence="Apache-2.0", ledger="MOD-042", note="Tesseract LSTM best, Urdu"),
        dict(url="https://github.com/google/fonts/raw/main/ofl/notonastaliqurdu/NotoNastaliqUrdu%5Bwght%5D.ttf", rel="fonts/NotoNastaliqUrdu.ttf", licence="OFL-1.1", ledger="", note="font for generated Urdu pages"),
        dict(url="https://github.com/google/fonts/raw/main/ofl/notonaskharabic/NotoNaskhArabic%5Bwght%5D.ttf", rel="fonts/NotoNaskhArabic.ttf", licence="OFL-1.1", ledger="", note="font for generated Urdu pages"),
        dict(url="https://github.com/google/fonts/raw/main/ofl/notosans/NotoSans%5Bwdth%2Cwght%5D.ttf", rel="fonts/NotoSans.ttf", licence="OFL-1.1", ledger="", note="font for generated English pages"),
    ],
    "audio": (
        [dict(url=hf("datasets", "google/fleurs", f"parquet-data/{lang}/test-00000-of-00001.parquet"), rel=f"fleurs/{lang}-test.parquet", licence="CC BY 4.0 (hub tag)", ledger="DAT-004",
              note="FLEURS read speech with transcripts, test split") for lang in ("ur_pk", "en_us")]
        + [dict(url=hf("models", repo, name), rel=f"models/asr/{repo.split('/')[1]}/{name}", licence="MIT (hub tag)", ledger="MOD-035", note="faster-whisper CTranslate2 weights")
           for repo, names in (
               ("Systran/faster-whisper-small", ("model.bin", "config.json", "tokenizer.json", "vocabulary.txt")),
               ("deepdml/faster-whisper-large-v3-turbo-ct2", ("model.bin", "config.json", "tokenizer.json", "vocabulary.json", "preprocessor_config.json")))
           for name in names]
    ),
    "audio_extra": (
        [dict(url=hf("datasets", "google/fleurs", f"parquet-data/{lang}/test-00000-of-00001.parquet"), rel=f"fleurs/{lang}-test.parquet", licence="CC BY 4.0 (hub tag)", ledger="DAT-004",
              note="FLEURS read speech with transcripts, test split") for lang in ("pa_in", "ps_af")]
        + [dict(url=hf("models", "Systran/faster-whisper-medium", name), rel=f"models/asr/faster-whisper-medium/{name}", licence="MIT (hub tag)", ledger="MOD-035", note="faster-whisper CTranslate2 weights")
           for name in ("model.bin", "config.json", "tokenizer.json", "vocabulary.txt")]
    ),
    "image_models": [
        dict(url=hf("models", "facebook/dinov2-small", name), rel=f"models/image/dinov2-small/{name}", licence="Apache-2.0 (hub tag)", ledger="MOD-038", note="DINOv2 small image embeddings")
        for name in ("config.json", "model.safetensors", "preprocessor_config.json")
    ],
    "ocr_onnx": [
        dict(url=hf("models", repo, name), rel=f"models/ocr/{repo.split('/')[1]}/{name}", licence="Apache-2.0 (hub tag)", ledger="MOD-039", note="PaddleOCR PP-OCRv5 ONNX export")
        for repo in ("PaddlePaddle/arabic_PP-OCRv5_mobile_rec_onnx", "PaddlePaddle/latin_PP-OCRv5_mobile_rec_onnx", "PaddlePaddle/en_PP-OCRv5_mobile_rec_onnx",
                     "PaddlePaddle/PP-OCRv5_mobile_det_onnx", "PaddlePaddle/PP-OCRv5_server_det_onnx")
        for name in ("inference.onnx", "inference.yml")
    ],
    "face_models": [
        dict(url=hf("models", "opencv/face_detection_yunet", name), rel=f"models/face/yunet/{name}", licence="MIT (OpenCV Zoo LICENSE)", ledger="MOD-008", note="YuNet face detector (OpenCV Zoo), the detector inside the worker's face role")
        for name in ("face_detection_yunet_2023mar.onnx", "LICENSE")
    ],
}


def sha256_file(path):
    digest = hashlib.sha256()
    with open(path, "rb") as source:
        for block in iter(lambda: source.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


def manifest_path():
    return os.path.join(ROOT, "manifest.jsonl")


def read_manifest():
    entries = {}
    if os.path.exists(manifest_path()):
        with open(manifest_path(), encoding="utf-8") as handle:
            for line in handle:
                if line.strip():
                    item = json.loads(line)
                    entries[item["rel"]] = item
    return entries


def download(url, dest):
    """Resumable download that refuses a short read: the byte count must equal what the server announced, otherwise the part file is kept and the caller retries."""
    part = dest + ".part"
    start = os.path.getsize(part) if os.path.exists(part) else 0
    request = urllib.request.Request(url, headers={**UA, **({"Range": f"bytes={start}-"} if start else {})})
    try:
        response = urllib.request.urlopen(request, timeout=60)
    except urllib.error.HTTPError as error:
        if error.code == 416:  # the part file is already complete
            os.replace(part, dest)
            return
        raise
    announced = int(response.headers.get("Content-Length") or 0)
    mode = "ab" if start and response.status == 206 else "wb"
    done = start if mode == "ab" else 0
    expected = (start + announced) if mode == "ab" and announced else announced
    last = time.time()
    with open(part, mode) as out:
        while True:
            block = response.read(1 << 20)
            if not block:
                break
            out.write(block)
            done += len(block)
            if time.time() - last > 15:
                print(f"    {done / 1e6:.0f} MB ...", flush=True)
                last = time.time()
    if expected and os.path.getsize(part) != expected:
        raise OSError(f"short read: {os.path.getsize(part)} of {expected} bytes")
    os.replace(part, dest)


def fetch_group(name):
    manifest = read_manifest()
    for item in GROUPS[name]:
        dest = os.path.join(ROOT, item["rel"])
        os.makedirs(os.path.dirname(dest), exist_ok=True)
        recorded = manifest.get(item["rel"])
        if recorded and os.path.exists(dest):
            if os.path.getsize(dest) == recorded["bytes"]:
                print(f"have   {item['rel']} ({recorded['bytes'] / 1e6:.1f} MB)")
                continue
            sys.exit(f"REFUSED {item['rel']}: the file differs from the manifest; nothing is replaced silently")
        print(f"fetch  {item['rel']}  <- {item['url']}", flush=True)
        for attempt in range(1, 9):  # a read timeout resumes from the part file
            try:
                download(item["url"], dest)
                break
            except (TimeoutError, OSError) as error:
                print(f"    interrupted ({error.__class__.__name__}), retry {attempt}", flush=True)
                time.sleep(3 * attempt)
        else:
            sys.exit(f"gave up on {item['rel']}")
        entry = {"rel": item["rel"], "url": item["url"], "bytes": os.path.getsize(dest), "sha256": sha256_file(dest), "licence": item["licence"],
                 "ledger": item["ledger"], "note": item["note"], "fetched": time.strftime("%Y-%m-%dT%H:%M:%S")}
        with open(manifest_path(), "a", encoding="utf-8") as handle:
            handle.write(json.dumps(entry) + "\n")
        print(f"       {entry['bytes'] / 1e6:.1f} MB sha256 {entry['sha256'][:16]}")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("groups", nargs="*")
    parser.add_argument("--list", action="store_true")
    args = parser.parse_args()
    if args.list or not args.groups:
        for name, items in GROUPS.items():
            print(f"{name}: {len(items)} files")
            for item in items:
                print(f"   {item['rel']:42} {item['licence']:22} {item['ledger']:8} {item['note']}")
        return
    os.makedirs(ROOT, exist_ok=True)
    for name in args.groups:
        fetch_group(name)


if __name__ == "__main__":
    main()
