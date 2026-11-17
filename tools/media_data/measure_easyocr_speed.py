#!/usr/bin/env python3
"""Speed and memory of the PRODUCT EasyOCR processor (ingestion/forensic_records/easyocr_ocr_processor.py), on DEVELOPMENT rows only (plan docs/roadmap/ocr-easyocr-integration-plan-20261117.md, 4.2).

It builds the processor through from_environment(), so the pinned-hash start, the thread setting (FORENSIC_EASYOCR_THREADS) and the size cap (FORENSIC_EASYOCR_MAX_SIDE) are the ones the worker would use.
Peak resident memory is process-wide and only grows, so every configuration runs in its own process. The driver mode starts them one after another and prints a table.

    docker run --rm --network none --memory 3000m -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro --entrypoint python nexusai-media-eval-easyocr:20261117 \
        /repo/tools/media_data/measure_easyocr_speed.py --driver --threads 1 2 4 8 --caps 0 2400 1800 1400 --cap-threads 4 --out /data/results/ocr/easyocr-speed-dev-20261117.json

--caps are tried at --cap-threads only; each cap is compared with cap 0 (no resizing) on the same images, character error as the mean of per-image rates after the normalisation of ocr_eval.py.
"""
import argparse
import hashlib
import json
import os
import resource
import statistics
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


def items(per_group):
    rows = []
    for folder, group, selector in (("ocr_en", "A4 English page", lambda r: True), ("ocr_ur", "A4 Urdu page", lambda r: r["kind"] == "page"), ("ocr_mixed", "mixed form/statement", lambda r: r["kind"] in ("form", "statement"))):
        lines = [json.loads(line) for line in open(Path(DATA) / "generated" / folder / "truth.jsonl", encoding="utf-8")]
        chosen = [r for r in lines if r["partition"] == "dev" and selector(r) and r["variant"] == "clean"][:per_group]
        rows += [{"group": group, "id": r["id"], "path": str(Path(DATA) / "generated" / folder / r["file"]), "text": r["text"]} for r in chosen]
    import nastaliq_lab
    tmp = Path(tempfile.mkdtemp())
    for n, (i, key, image, reference) in enumerate(nastaliq_lab.rows("dev")):
        if n >= per_group:
            break
        path = tmp / f"unb-{i}.png"
        path.write_bytes(image)
        rows.append({"group": "newspaper paragraph", "id": f"unb-{i}", "path": str(path), "text": reference})
    return rows


def worker(args):
    import ocr_eval
    os.environ.update({"FORENSIC_EASYOCR_MODEL_DIR": MODEL_DIR, "FORENSIC_EASYOCR_THREADS": str(args.threads), "FORENSIC_EASYOCR_MAX_SIDE": str(args.cap),
                       "FORENSIC_EASYOCR_DETECTOR_SHA256": sha256(f"{MODEL_DIR}/craft_mlt_25k.pth"), "FORENSIC_EASYOCR_RECOGNIZER_SHA256": sha256(f"{MODEL_DIR}/arabic.pth")})
    from easyocr_ocr_processor import EasyOcrProcessor
    started = time.time()
    processor = EasyOcrProcessor.from_environment()
    loaded = time.time() - started
    records = []
    for item in items(args.per_group):
        t0 = time.time()
        result = processor.process_image(Path(item["path"]), source_file=Path(item["path"]).name, source_sha256="0" * 64, evidence_id="e", version_id="v")
        seconds = time.time() - t0
        text = "\n".join(o.payload["raw_text"] for o in result.observations)
        records.append({"group": item["group"], "id": item["id"], "seconds": round(seconds, 2), "cer": ocr_eval.cer(item["text"], text), "lines": len(result.observations)})
    print(json.dumps({"threads": args.threads, "cap": args.cap, "load_seconds": round(loaded, 1), "peak_rss_mb": round(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024), "rows": records}), flush=True)
    return 0


def driver(args):
    configs = [(t, 0) for t in args.threads] + [(args.cap_threads, c) for c in args.caps if c]
    results = []
    for threads, cap in configs:
        done = subprocess.run([sys.executable, __file__, "--threads", str(threads), "--cap", str(cap), "--per-group", str(args.per_group)], capture_output=True, text=True)
        line = [l for l in done.stdout.splitlines() if l.startswith("{")]
        if done.returncode != 0 or not line:
            results.append({"threads": threads, "cap": cap, "error": (done.stderr or done.stdout)[-400:]})
            print(f"threads {threads} cap {cap}: FAILED {(done.stderr or '')[-200:]}", flush=True)
            continue
        record = json.loads(line[-1])
        results.append(record)
        groups = {}
        for r in record["rows"]:
            groups.setdefault(r["group"], []).append(r)
        summary = " | ".join(f"{g}: {statistics.mean(x['seconds'] for x in rs):.1f} s, CER {100 * statistics.mean(x['cer'] for x in rs):.1f}%" for g, rs in groups.items())
        print(f"threads {threads} cap {cap or 'none'}: load {record['load_seconds']} s, peak {record['peak_rss_mb']} MB | {summary}", flush=True)
    json.dump(results, open(args.out, "w", encoding="utf-8"), indent=1)
    return 0


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--driver", action="store_true")
    ap.add_argument("--threads", type=int, nargs="+", default=[1])
    ap.add_argument("--caps", type=int, nargs="*", default=[])
    ap.add_argument("--cap-threads", type=int, default=4)
    ap.add_argument("--cap", type=int, default=0)
    ap.add_argument("--per-group", type=int, default=3)
    ap.add_argument("--out", default="")
    args = ap.parse_args(argv)
    if args.driver:
        if not args.out:
            raise SystemExit("--out is required with --driver")
        return driver(args)
    args.threads = args.threads[0] if isinstance(args.threads, list) else args.threads
    return worker(args)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
