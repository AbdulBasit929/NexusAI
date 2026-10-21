#!/usr/bin/env python3
"""Probe candidate public datasets and models on the Hugging Face hub and record what the hub says about each: licence tag, gated or not, size of the files, last change.

Read-only: it calls the public API (no login, no download of data). The answer is evidence for the licence ledger and the decision rules in docs/data/media-datasets-and-models-20261021.md;
a licence tag on a hub page is the uploader's claim, so a row stays `to_verify` until its card and its source have been read.

    python -X utf8 tools/media_data/probe_sources.py                      # the built-in candidate list
    python -X utf8 tools/media_data/probe_sources.py --search "urdu ocr"  # list datasets matching a query (name, licence, downloads)
    python -X utf8 tools/media_data/probe_sources.py --search "license plate" --kind models
"""
import argparse
import json
import sys
import urllib.parse
import urllib.request

API = "https://huggingface.co/api"

# role -> (kind, repo id): what each track would use or compare. Only ids worth a look; nothing here is downloaded.
CANDIDATES = {
    "A speech data": [
        ("datasets", "google/fleurs"),
        ("datasets", "mozilla-foundation/common_voice_17_0"),
        ("datasets", "mozilla-foundation/common_voice_13_0"),
        ("datasets", "facebook/voxpopuli"),
        ("datasets", "ai4bharat/Svarah"),
    ],
    "A speech models": [
        ("models", "openai/whisper-large-v3-turbo"),
        ("models", "openai/whisper-large-v3"),
        ("models", "openai/whisper-medium"),
        ("models", "Systran/faster-whisper-large-v3"),
        ("models", "Systran/faster-whisper-medium"),
        ("models", "Systran/faster-whisper-small"),
        ("models", "deepdml/faster-whisper-large-v3-turbo-ct2"),
        ("models", "kingabzpro/whisper-large-v3-turbo-urdu"),
        ("models", "awaaz-se-alfaaz/whisper-turbo-v3-urdu"),
        ("models", "BuzzASR/urdu"),
        ("models", "ai4bharat/indic-conformer-600m-multilingual"),
        ("models", "facebook/omnilingual-asr-300m"),
        ("models", "facebook/seamless-m4t-v2-large"),
        ("models", "snakers4/silero-vad"),
        ("models", "pyannote/speaker-diarization-community-1"),
        ("models", "speechbrain/lang-id-voxlingua107-ecapa"),
        ("models", "facebook/mms-lid-126"),
    ],
    "D/O document and Urdu OCR data": [
        ("datasets", "naver-clova-ix/cord-v2"),
        ("datasets", "nielsr/funsd"),
        ("datasets", "pierreguillou/DocLayNet-small"),
        ("datasets", "ds4sd/DocLayNet"),
        ("datasets", "jinhybr/OCR-Receipt"),
    ],
    "D/O OCR and layout models": [
        ("models", "PaddlePaddle/arabic_PP-OCRv5_mobile_rec"),
        ("models", "PaddlePaddle/PP-OCRv5_server_det"),
        ("models", "PaddlePaddle/PaddleOCR-VL"),
        ("models", "microsoft/trocr-base-printed"),
        ("models", "Qwen/Qwen3-VL-2B-Instruct"),
        ("models", "Qwen/Qwen3-VL-4B-Instruct"),
        ("models", "ibm-granite/granite-docling-258M"),
        ("models", "docling-project/docling-layout-heron"),
        ("models", "BAAI/bge-m3"),
        ("models", "BAAI/bge-reranker-v2-m3"),
    ],
    "I image models": [
        ("models", "google/siglip-base-patch16-224"),
        ("models", "google/siglip2-base-patch16-224"),
        ("models", "facebook/dinov2-small"),
        ("models", "facebook/dinov2-base"),
        ("models", "openai/clip-vit-base-patch32"),
    ],
    "P/V plate data and models": [
        ("datasets", "keremberke/license-plate-object-detection"),
        ("datasets", "Voxel51/OpenImages-V7-license-plates"),
        ("models", "keremberke/yolov8n-license-plate"),
        ("models", "justjuu/rtdetr-v2-license-plate-detection"),
        ("models", "autolane/rfdetr-alpr"),
        ("models", "roboflow/rf-detr"),
        ("models", "ankandrew/open-image-models"),
        ("models", "ankandrew/cct-xs-v2-global-model"),
        ("models", "PekingU/rtdetr_v2_r18vd"),
        ("models", "PaddlePaddle/PP-OCRv5_mobile_det"),
    ],
    "F face models": [
        ("models", "opencv/face_detection_yunet"),
        ("models", "opencv/face_recognition_sface"),
        ("models", "fal/AuraFace-v1"),
        ("models", "arnabdhar/YOLOv8-Face-Detection"),
    ],
}


def get(url):
    request = urllib.request.Request(url, headers={"User-Agent": "nexusai-media-probe/1"})
    with urllib.request.urlopen(request, timeout=45) as response:
        return json.load(response)


def summarise(kind, repo):
    try:
        data = get(f"{API}/{kind}/{repo}?blobs=true")
    except Exception as error:  # a missing id is an answer, not a failure
        return {"id": repo, "kind": kind, "error": str(error)[:80]}
    tags = data.get("tags", [])
    licence = next((t.split(":", 1)[1] for t in tags if t.startswith("license:")), (data.get("cardData") or {}).get("license", ""))
    siblings = data.get("siblings") or []
    size = sum((s.get("size") or 0) for s in siblings)
    return {"id": repo, "kind": kind, "license": licence or "(none stated)", "gated": data.get("gated"), "private": data.get("private"),
            "size_mb": round(size / 1e6, 1) if size else None, "files": len(siblings), "downloads": data.get("downloads"), "modified": (data.get("lastModified") or "")[:10]}


def search(query, kind, limit):
    url = f"{API}/{kind}?search={urllib.parse.quote(query)}&limit={limit}&sort=downloads&direction=-1&full=true"
    rows = []
    for item in get(url):
        tags = item.get("tags", [])
        licence = next((t.split(":", 1)[1] for t in tags if t.startswith("license:")), "(none stated)")
        rows.append({"id": item.get("id"), "license": licence, "gated": item.get("gated"), "downloads": item.get("downloads"), "modified": (item.get("lastModified") or "")[:10]})
    return rows


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--search")
    parser.add_argument("--kind", default="datasets", choices=("datasets", "models"))
    parser.add_argument("--limit", type=int, default=25)
    parser.add_argument("--json", help="write the table of the built-in list to this file")
    args = parser.parse_args()
    if args.search:
        for row in search(args.search, args.kind, args.limit):
            print(f"{row['id']:60} {str(row['license']):22} gated={str(row['gated']):6} dl={row['downloads']} {row['modified']}")
        return
    out = {}
    for group, items in CANDIDATES.items():
        print(f"\n== {group}")
        out[group] = []
        for kind, repo in items:
            row = summarise(kind, repo)
            out[group].append(row)
            if "error" in row:
                print(f"  {repo:58} NOT FOUND ({row['error']})")
            else:
                print(f"  {repo:58} {str(row['license']):22} gated={str(row['gated']):6} {str(row['size_mb']) + ' MB':>11} {row['modified']}")
    if args.json:
        with open(args.json, "w", encoding="utf-8") as handle:
            json.dump(out, handle, indent=1)
        print(f"\nwritten {args.json}")


if __name__ == "__main__":
    sys.exit(main())
