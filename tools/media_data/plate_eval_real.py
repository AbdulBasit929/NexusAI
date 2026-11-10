#!/usr/bin/env python3
"""Real-photo plate evaluation on the human-verified Pakistani gold (docs/data/plates-m5-preregistration-20261110.md, Amendment 1).

    docker run --rm --network none --cpus 4 -m 1500m --entrypoint python \
      -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro -v <nxmmr models>:/nxmmr:ro -v "<Downloads>/archive/Pakistani License Number Plates Data":/real:ro \
      -v <human-verification>:/gold:ro nexusai-media-eval:latest /repo/tools/media_data/plate_eval_real.py --engine incumbent --split development

Engines: `incumbent` (the FastALPR pair that runs today; reproduces the earlier benchmark), `reader_on_incumbent_boxes` (our reader on crops from the incumbent detector's boxes: evaluation
scaffolding only, separates a reader gain from a detector gain), `nexus` (our detector and our reader). Scoring is the earlier evaluator's own (scripts/benchmark_nexusai_nxmmr_anpr.py):
plate strings normalised to letters and digits, an image's truth plate counts as exact if any predicted plate equals it, detection is a count proxy (the gold has no boxes), character errors by
greedy pairing, Wilson 95% intervals.

PRIVACY: the labels are marked local, not to be committed or published. This script never prints a plate string; the per-image file (with strings) is written only to --out, which must be outside
the repository (default /data/results/plates in the local media-data folder). Stdout and any committed report carry aggregates only.
"""
import argparse
import csv
import json
import os
import sys
import time
from collections import Counter
from pathlib import Path

import cv2

sys.path.insert(0, os.environ.get("NEXUSAI_INGEST", "/repo/ingestion/forensic_records"))
import plate_formats as pf  # noqa: E402
import plate_reader as pr  # noqa: E402


def normalize(value):
    return "".join(c for c in (value or "").upper() if c.isalnum())


def edit_distance(a, b):
    prior = list(range(len(b) + 1))
    for i, ca in enumerate(a, 1):
        current = [i]
        for j, cb in enumerate(b, 1):
            current.append(min(current[-1] + 1, prior[j] + 1, prior[j - 1] + (ca != cb)))
        prior = current
    return prior[-1]


def greedy_pairs(truth, predictions):
    truth, predictions, pairs = list(truth), list(predictions), []
    while truth and predictions:
        _, ti, pi = min((edit_distance(t, p), i, j) for i, t in enumerate(truth) for j, p in enumerate(predictions))
        pairs.append((truth.pop(ti), predictions.pop(pi)))
    return pairs, truth, predictions


def score(truth_raw, prediction_raw):
    truth = [normalize(v) for v in truth_raw]
    predictions = [normalize(v) for v in prediction_raw if normalize(v)]
    exact = sum((Counter(truth) & Counter(predictions)).values())
    pairs, missing, _ = greedy_pairs(truth, predictions)
    errors = sum(edit_distance(a, b) for a, b in pairs) + sum(len(m) for m in missing)
    return {"truth": len(truth), "predicted": len(predictions), "detected": min(len(truth), len(predictions)), "exact": exact, "char_errors": errors, "chars": sum(len(t) for t in truth)}


def wilson(k, n, z=1.959963984540054):
    if n == 0:
        return None
    p = k / n
    d = 1 + z * z / n
    centre = (p + z * z / (2 * n)) / d
    half = z * ((p * (1 - p) / n + z * z / (4 * n * n)) ** 0.5) / d
    return [round(centre - half, 4), round(centre + half, 4)]


def incumbent_engine(args):
    from fast_alpr import ALPR
    from fast_alpr.default_ocr import DefaultOCR
    from open_image_models import create_detector
    providers = ["CPUExecutionProvider"]
    detector = create_detector(args.incumbent_detector, backend="yolo_v9", class_labels={0: "license_plate"}, conf_thresh=0.75, providers=providers)
    ocr = DefaultOCR(hub_ocr_model=None, device="cpu", providers=providers, model_path=args.incumbent_ocr, config_path=args.incumbent_ocr_config, force_download=False)
    return ALPR(detector=detector, ocr=ocr)


def read_with_ours(reader, crop):
    candidates = reader.read(crop)
    text, fmt, flags = pf.choose(candidates)
    return text, fmt, flags, reader.confidence(candidates)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--engine", choices=["incumbent", "reader_on_incumbent_boxes", "rfdetr_pair", "nexus"], required=True)
    parser.add_argument("--split", choices=["development", "sealed_holdout"], required=True)
    parser.add_argument("--labels", default="/gold/image-labels.csv")
    parser.add_argument("--root", default="/real")
    parser.add_argument("--reader", default="/data/models/plates/reader-v1/plate-reader-v1.onnx")
    parser.add_argument("--incumbent-detector", default="/nxmmr/yolo-v9-t-384-license-plates-end2end.onnx")
    parser.add_argument("--incumbent-ocr", default="/nxmmr/cct_xs_v2_global.onnx")
    parser.add_argument("--incumbent-ocr-config", default="/nxmmr/cct_xs_v2_global_plate_config.yaml")
    parser.add_argument("--detector-onnx", default="/data/models/plates/rfdetr-alpr/rfdetr_alpr.onnx")
    parser.add_argument("--detector-norm", default="imagenet", choices=["imagenet", "unit"])
    parser.add_argument("--detector-threshold", type=float, default=0.5)
    parser.add_argument("--drop-non-plates", action="store_true")
    parser.add_argument("--out", default="/data/results/plates")
    parser.add_argument("--tag", default="")
    args = parser.parse_args()
    if args.engine == "nexus":
        sys.exit("the nexus detector is not built yet; use reader_on_incumbent_boxes")
    rows = [r for r in csv.DictReader(open(args.labels, encoding="utf-8-sig")) if r["split"] == args.split]
    detector = ocr_model = None
    if args.engine == "rfdetr_pair":
        import plate_detector_onnx as pd
        detector = pd.OnnxPlateDetector(args.detector_onnx, cv2, normalise=args.detector_norm, threshold=args.detector_threshold, drop_non_plates=args.drop_non_plates)
        from fast_alpr.default_ocr import DefaultOCR
        ocr_model = DefaultOCR(hub_ocr_model=None, device="cpu", providers=["CPUExecutionProvider"], model_path=args.incumbent_ocr, config_path=args.incumbent_ocr_config, force_download=False)
        engine = None
    else:
        engine = incumbent_engine(args)
    reader = pr.OnnxPlateReader(args.reader, cv2) if args.engine == "reader_on_incumbent_boxes" else None
    per_image, totals, latencies, flagged = [], Counter(), [], Counter()
    for row in rows:
        frame = cv2.imread(str(Path(args.root) / row["relative_file"]))
        if frame is None:
            sys.exit("an image could not be read")
        started = time.time()
        predictions, notes = [], []
        if detector is not None:
            for x1, y1, x2, y2, det_score in detector.detect(frame)[:32]:
                if x2 - x1 < 4 or y2 - y1 < 4:
                    continue
                read = ocr_model.predict(frame[y1:y2, x1:x2])
                predictions.append(read.text if read is not None else "")
                notes.append({"detector_score": round(det_score, 3)})
            results = []
        else:
            results = engine.predict(frame)
        for result in results[:32]:
            if float(result.detection.confidence) < 0.75:
                continue
            if reader is None:
                predictions.append(result.ocr.text if result.ocr is not None else "")
                continue
            b = result.detection.bounding_box
            x1, y1, x2, y2 = max(0, int(b.x1)), max(0, int(b.y1)), min(frame.shape[1], int(b.x2)), min(frame.shape[0], int(b.y2))
            if x2 <= x1 or y2 <= y1:
                continue
            text, fmt, flags, confidence = read_with_ours(reader, frame[y1:y2, x1:x2])
            predictions.append(text)
            notes.append({"format": fmt, "flags": flags, "confidence": round(confidence, 3)})
            for flag in flags:
                flagged[flag] += 1
            if fmt is None:
                flagged["no_format"] += 1
        latencies.append(time.time() - started)
        s = score([row["plate_text"]], predictions)
        totals.update(s)
        per_image.append({"sample_id": row["sample_id"], "difficulty": row["difficulty"], "predicted": predictions, "notes": notes, **s})
    n = len(rows)
    summary = {
        "engine": args.engine, "split": args.split, "images": n, "plates_found_count_proxy": [totals["detected"], n], "plates_found_wilson95": wilson(totals["detected"], n),
        "exact_normalised": [totals["exact"], n], "exact_rate": round(totals["exact"] / n, 4), "exact_wilson95": wilson(totals["exact"], n),
        "character_error": round(totals["char_errors"] / max(1, totals["chars"]), 4), "false_reads": totals["predicted"] - totals["exact"],
        "seconds_per_image_mean": round(sum(latencies) / n, 3), "reader_flags": dict(flagged), "model_inputs": {"reader": args.reader if reader else None},
    }
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    stamp = time.strftime("%Y%m%d-%H%M%S")
    (out / f"real-{args.engine}-{args.split}{args.tag}-{stamp}.json").write_text(json.dumps({"summary": summary, "per_image_PRIVATE": per_image}, indent=1), encoding="utf-8")
    print(json.dumps(summary, indent=1))


if __name__ == "__main__":
    main()
