#!/usr/bin/env python3
"""A 300 dpi scan is twice the side of the 150 dpi pages the speed run used (2480 x 3508 against 1240 x 1754). This measures what the product EasyOCR processor costs on such an image, with and without the
size cap (FORENSIC_EASYOCR_MAX_SIDE), on DEVELOPMENT rows only: one generated Urdu page and one generated mixed form, each enlarged 2x with cubic interpolation (the content is the same; only the pixel count changes).
Each configuration runs in its own process because peak resident memory only grows.

    docker run --rm --network none --memory 5000m -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro --entrypoint python nexusai-media-eval-easyocr:20261117 \
        /repo/tools/media_data/measure_easyocr_large.py --driver --caps 0 1800 --threads 4 --out /data/results/ocr/easyocr-large-dev-20261117.json
"""
import argparse
import hashlib
import json
import os
import resource
import subprocess
import sys
import tempfile
import time
from pathlib import Path

sys.path.insert(0, "/repo/tools/media_data")
sys.path.insert(0, "/repo/ingestion/forensic_records")
DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
MODEL_DIR = f"{DATA}/models/ocr/easyocr/model"


def sha256(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def images():
    import cv2
    out, tmp = [], Path(tempfile.mkdtemp())
    for folder, kind in (("ocr_ur", "page"), ("ocr_mixed", "form")):
        rows = [json.loads(line) for line in open(Path(DATA) / "generated" / folder / "truth.jsonl", encoding="utf-8")]
        row = next(r for r in rows if r["partition"] == "dev" and r["kind"] == kind and r["variant"] == "clean")
        frame = cv2.imread(str(Path(DATA) / "generated" / folder / row["file"]))
        big = cv2.resize(frame, None, fx=2, fy=2, interpolation=cv2.INTER_CUBIC)
        path = tmp / f"{row['id']}-2x.png"
        cv2.imwrite(str(path), big)
        out.append({"id": row["id"], "path": str(path), "text": row["text"], "shape": big.shape[:2]})
    return out


def worker(args):
    import ocr_eval
    os.environ.update({"FORENSIC_EASYOCR_MODEL_DIR": MODEL_DIR, "FORENSIC_EASYOCR_THREADS": str(args.threads), "FORENSIC_EASYOCR_MAX_SIDE": str(args.cap),
                       "FORENSIC_EASYOCR_DETECTOR_SHA256": sha256(f"{MODEL_DIR}/craft_mlt_25k.pth"), "FORENSIC_EASYOCR_RECOGNIZER_SHA256": sha256(f"{MODEL_DIR}/arabic.pth")})
    from easyocr_ocr_processor import EasyOcrProcessor
    processor = EasyOcrProcessor.from_environment()
    after_load = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss // 1024
    rows = []
    for item in images():
        t0 = time.time()
        result = processor.process_image(Path(item["path"]), source_file="x.png", source_sha256="0" * 64, evidence_id="e", version_id="v")
        text = "\n".join(o.payload["raw_text"] for o in result.observations)
        rows.append({"id": item["id"], "shape": item["shape"], "seconds": round(time.time() - t0, 1), "cer": ocr_eval.cer(item["text"], text)})
    print(json.dumps({"cap": args.cap, "threads": args.threads, "peak_after_load_mb": after_load, "peak_mb": resource.getrusage(resource.RUSAGE_SELF).ru_maxrss // 1024, "rows": rows}), flush=True)
    return 0


def driver(args):
    results = []
    for cap in args.caps:
        done = subprocess.run([sys.executable, __file__, "--cap", str(cap), "--threads", str(args.threads)], capture_output=True, text=True)
        line = [l for l in done.stdout.splitlines() if l.startswith("{")]
        record = json.loads(line[-1]) if line and done.returncode == 0 else {"cap": cap, "error": (done.stderr or done.stdout)[-300:]}
        results.append(record)
        print(json.dumps(record), flush=True)
    json.dump(results, open(args.out, "w", encoding="utf-8"), indent=1)
    return 0


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--driver", action="store_true")
    ap.add_argument("--caps", type=int, nargs="*", default=[0, 1800])
    ap.add_argument("--cap", type=int, default=0)
    ap.add_argument("--threads", type=int, default=4)
    ap.add_argument("--out", default="")
    args = ap.parse_args(argv)
    return driver(args) if args.driver else worker(args)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
