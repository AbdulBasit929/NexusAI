#!/usr/bin/env python3
"""Router threshold measurement (docs/roadmap/ocr-easyocr-integration-plan-20261117.md, section 4.1): the worker's own script probe (the PP-OCRv5 Arabic recogniser on the longest text lines,
OnnxPaddleOCRProcessor.arabic_script_share) is run on the DEVELOPMENT rows of the English pages, the receipts, the Urdu pages and the mixed Urdu-and-English pages, and the Arabic-script share is
recorded per image. No sealed row is read. The threshold T_low (FORENSIC_OCR_ROUTER_URDU_SHARE) is the value that

  * sends at least 98% of the English pages and of the receipts to the Latin route (share below T_low),
  * sends at least 90% of the mixed pages and at least 98% of the Urdu pages to EasyOCR (share at or above T_low).

If no value meets all four, the best compromise (largest smallest margin) is reported with the numbers, and nothing is claimed.

    docker run --rm --network none --memory 2000m -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro nexusai-media-eval:latest \
        python /repo/tools/media_data/measure_router_share.py --out /data/results/ocr/router-share-dev-20261117.json
"""
import argparse
import hashlib
import json
import os
import sys
import time
from pathlib import Path

sys.path.insert(0, "/repo/ingestion/forensic_records")
DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")


def partition(key):
    return "dev" if hashlib.sha256(key.encode()).hexdigest()[-1] in "0123456789ab" else "sealed"


def build_probe_processor():
    import onnx_ocr_processor as op
    import onnx_text_ocr as engine
    base = Path(DATA) / "models" / "ocr"

    def recogniser(name, arabic):
        folder = base / f"{name}_PP-OCRv5_mobile_rec_onnx"
        return engine.PaddleOnnxRecognizer(folder / "inference.onnx", folder / "inference.yml", arabic_script=arabic)
    detector = engine.PaddleOnnxDetector(base / "PP-OCRv5_mobile_det_onnx" / "inference.onnx")
    return op.OnnxPaddleOCRProcessor(detector, {"latin": recogniser("latin", False)}, model_hashes={}, script_probe=recogniser("arabic", True))


def generated(folder, name, wanted):
    rows = [json.loads(line) for line in open(Path(DATA) / "generated" / folder / "truth.jsonl", encoding="utf-8")]
    for row in rows:
        if row["partition"] == "dev" and wanted(row):
            yield {"set": name, "group": row.get("kind") or "page", "style": row.get("style"), "variant": row.get("variant"), "id": row["id"], "path": Path(DATA) / "generated" / folder / row["file"]}


def cord_dev(limit):
    import pyarrow.parquet as pq
    tmp = Path("/tmp/cord_share")
    tmp.mkdir(exist_ok=True)
    parquet = pq.ParquetFile(Path(DATA) / "cord" / "test.parquet")
    wanted = [i for i in range(parquet.metadata.num_rows) if partition(f"cord-{i}") == "dev"][:limit]
    chosen, index = set(wanted), 0
    for batch in parquet.iter_batches(batch_size=4, columns=["image"]):
        for record in batch.to_pylist():
            if index in chosen:
                image = record["image"]
                path = tmp / f"cord-{index}.png"
                path.write_bytes(image["bytes"] if isinstance(image, dict) else image)
                yield {"set": "cord", "group": "receipt", "style": None, "variant": None, "id": f"cord-{index}", "path": path}
            index += 1


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--out", required=True)
    ap.add_argument("--cord-limit", type=int, default=60)
    args = ap.parse_args(argv)
    processor = build_probe_processor()
    items = list(generated("ocr_en", "en_pages", lambda r: True)) + list(generated("ocr_ur", "ur_pages", lambda r: r["kind"] == "page")) + list(generated("ocr_mixed", "mixed", lambda r: True))
    records = []
    started = time.time()
    for n, item in enumerate(items + list(cord_dev(args.cord_limit)), 1):
        t0 = time.time()
        share = processor.arabic_script_share(item["path"])
        records.append({**{k: v for k, v in item.items() if k != "path"}, "share": share, "seconds": round(time.time() - t0, 2)})
        if n % 50 == 0:
            print(f"{n} images, {time.time() - started:.0f} s", flush=True)
    json.dump(records, open(args.out, "w", encoding="utf-8"), ensure_ascii=False, indent=0)
    print(f"{len(records)} images -> {args.out}", flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
