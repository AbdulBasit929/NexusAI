#!/usr/bin/env python3
"""Can the ONNX Arabic recogniser tell an Arabic-script (Urdu) page from a Latin one? Detect text lines, read the longest few with the Arabic recogniser, and take the share of Arabic-script
letters among all letters it wrote. Urdu text gives a high share (the model writes Arabic-script letters, right or wrong); Latin text gives a low one. Measured on generated Urdu pages, generated
English pages and public receipt photographs (dev partitions).

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data:ro -v <repo>:/repo:ro nexusai-media-eval:latest python /repo/tools/media_data/script_share_eval.py
"""
import hashlib
import json
import os
import statistics as st
import sys
from pathlib import Path

import cv2
import numpy as np
import pyarrow.parquet as pq

sys.path.insert(0, "/repo/ingestion/forensic_records")
import onnx_text_ocr as engine

DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
base = Path(DATA) / "models" / "ocr"


def partition(key):
    return "dev" if hashlib.sha256(key.encode()).hexdigest()[-1] in "0123456789ab" else "sealed"


def arabic_share(text):
    letters = [c for c in text if c.isalpha()]
    if not letters:
        return None
    arabic = sum(1 for c in letters if "\u0600" <= c <= "\u06ff" or "\u0750" <= c <= "\u077f" or "\ufb50" <= c <= "\ufdff" or "\ufe70" <= c <= "\ufeff")
    return arabic / len(letters)


def main():
    detector = engine.PaddleOnnxDetector(base / "PP-OCRv5_mobile_det_onnx" / "inference.onnx")
    arabic = engine.PaddleOnnxRecognizer(base / "arabic_PP-OCRv5_mobile_rec_onnx" / "inference.onnx", base / "arabic_PP-OCRv5_mobile_rec_onnx" / "inference.yml", arabic_script=True)

    def page_share(image, lines=6):
        boxes = detector.detect(image)
        boxes = sorted(boxes, key=lambda q: -float(np.linalg.norm(q[0] - q[1])))[:lines]
        texts = [arabic.recognize(engine._crop_quad(image, q))[0] for q in boxes]
        return arabic_share(" ".join(texts))

    sets = {}
    rows = [json.loads(l) for l in open(Path(DATA) / "generated" / "ocr_ur" / "truth.jsonl", encoding="utf-8")]
    sets["urdu pages (expect high)"] = [cv2.imread(str(Path(DATA) / "generated" / "ocr_ur" / r["file"])) for r in rows if r["kind"] == "page" and r["partition"] == "dev"]
    rows = [json.loads(l) for l in open(Path(DATA) / "generated" / "ocr_en" / "truth.jsonl", encoding="utf-8")]
    sets["english pages (expect low)"] = [cv2.imread(str(Path(DATA) / "generated" / "ocr_en" / r["file"])) for r in rows if r["partition"] == "dev"][:64]
    table = pq.read_table(Path(DATA) / "cord" / "test.parquet")
    receipts = []
    for i in range(table.num_rows):
        if partition(f"cord-{i}") == "dev":
            data = table.slice(i, 1).to_pylist()[0]["image"]["bytes"]
            receipts.append(cv2.imdecode(np.frombuffer(data, np.uint8), cv2.IMREAD_COLOR))
    sets["receipts (expect low)"] = receipts[:64]
    for name, images in sets.items():
        shares = [page_share(image) for image in images]
        shares = [s for s in shares if s is not None]
        shares.sort()
        print(f"{name:28} n={len(shares):3}  min {shares[0]:.2f}  p10 {shares[len(shares) // 10]:.2f}  median {st.median(shares):.2f}  p90 {shares[(len(shares) * 9) // 10]:.2f}  max {shares[-1]:.2f}")


if __name__ == "__main__":
    main()
