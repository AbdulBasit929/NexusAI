#!/usr/bin/env python3
"""OCR measurement for track D (docs/data/media-activation-programme-20261021.md): engines against generated pages, Urdu lines and real receipts, on the dev partition.

Runs inside the evaluation image, network off, data read-only except the results folder:

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro nexusai-media-eval:latest \
        python /repo/tools/media_data/ocr_eval.py --set en_pages --engine tess:fast:eng:3 --engine tess:fast:eng+urd:11 --partition dev

Engine spec `tess:<fast|best>:<languages>:<psm>` runs the worker's own TesseractImageOCRProcessor (so the numbers are those of the code that would run in production) with the chosen language data.
Metrics: character error rate on the normalised text in the order the engine produced it (NFKC, whitespace collapsed, zero-width removed); bag-of-words F1 (order free); identifier exact match
(the planted string appears verbatim) for the generated pages; seconds per image. A result file per run is written to /data/results/ocr/.
"""
import argparse
import hashlib
import io
import json
import os
import re
import sys
import time
import unicodedata
from pathlib import Path

import editdistance

sys.path.insert(0, "/repo/ingestion/forensic_records")
DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
SORT_RTL = os.environ.get("OCR_SORT_RTL", "0") == "1"
ZERO_WIDTH = dict.fromkeys(map(ord, "\u200b\u200c\u200d\u200e\u200f\u2060\ufeff"), None)


def normalise(text):
    text = unicodedata.normalize("NFKC", text).translate(ZERO_WIDTH)
    return re.sub(r"\s+", " ", text).strip()


def cer(reference, hypothesis):
    ref = normalise(reference).replace(" ", "")
    hyp = normalise(hypothesis).replace(" ", "")
    if not ref:
        return 0.0 if not hyp else 1.0
    return editdistance.eval(ref, hyp) / len(ref)


def words(text):
    return re.findall(r"\S+", normalise(text))


def bag_f1(reference, hypothesis):
    from collections import Counter
    ref, hyp = Counter(words(reference)), Counter(words(hypothesis))
    overlap = sum((ref & hyp).values())
    if not ref or not hyp:
        return 0.0
    precision, recall = overlap / sum(hyp.values()), overlap / sum(ref.values())
    return 0.0 if precision + recall == 0 else 2 * precision * recall / (precision + recall)


def partition(key):
    return "dev" if hashlib.sha256(key.encode()).hexdigest()[-1] in "0123456789ab" else "sealed"


def make_tesseract(spec):
    _, data, languages, psm = spec.split(":")
    import media_pipeline
    folder = Path(DATA) / "tessdata" / data
    hashes = {}
    for language in languages.split("+"):
        file = folder / f"{language}.traineddata"
        hashes[language] = hashlib.sha256(file.read_bytes()).hexdigest()
    processor = media_pipeline.TesseractImageOCRProcessor(
        Path("/usr/bin/tesseract"), folder, languages=languages, tessdata_revision=f"eval-{data}", tessdata_hashes=hashes, page_segmentation_mode=int(psm), timeout_seconds=120)

    def run(path):
        result = processor.process_image(path, evidence_id="e", version_id="v", source_sha256="0" * 64, source_file=path.name)
        # reading order: lines top to bottom by their box (Tesseract's block order puts a left-hand table before right-hand Urdu text on a right-to-left page)
        ordered = sorted(result.observations, key=lambda o: (o.payload["bbox"]["y"] // 12, -o.payload["bbox"]["x"] if SORT_RTL else o.payload["bbox"]["x"]))
        lines = [o.payload["raw_text"] for o in ordered]
        confidences = [o.confidence for o in result.observations if o.confidence is not None]
        return "\n".join(lines), (sum(confidences) / len(confidences) if confidences else None)
    return run


def make_onnx(spec):
    """onnx:<arabic|latin|en|both>:<line|page>[:server]: the PP-OCRv5 ONNX exports through ingestion/forensic_records/onnx_text_ocr.py (line = recogniser only, page = detector + recogniser)."""
    import cv2
    import onnx_text_ocr as engine
    parts = spec.split(":")
    script, mode = parts[1], parts[2]
    base = Path(DATA) / "models" / "ocr"

    def recogniser(name, arabic):
        folder = base / f"{name}_PP-OCRv5_mobile_rec_onnx"
        return engine.PaddleOnnxRecognizer(folder / "inference.onnx", folder / "inference.yml", arabic_script=arabic)
    recognisers = {}
    if script in ("arabic", "both"):
        recognisers["arabic"] = recogniser("arabic", True)
    if script in ("latin", "both"):
        recognisers["latin"] = recogniser("latin", False)
    if script == "en":
        recognisers["latin"] = recogniser("en", False)
    if mode == "line":
        only = next(iter(recognisers.values()))

        def run_line(path):
            text, confidence = only.recognize(cv2.imread(str(path)))
            return text, confidence
        return run_line
    detector_dir = base / ("PP-OCRv5_server_det_onnx" if "server" in parts[3:] else "PP-OCRv5_mobile_det_onnx")
    ocr = engine.PaddleOnnxOCR(engine.PaddleOnnxDetector(detector_dir / "inference.onnx"), recognisers)

    def run_page(path):
        lines = ocr.read(cv2.imread(str(path)))
        confidence = sum(l.confidence for l in lines) / len(lines) if lines else None
        return engine.join_lines(lines), confidence
    return run_page


def make_engine(spec):
    kind = spec.split(":")[0]
    if kind == "tess":
        return make_tesseract(spec)
    if kind == "onnx":
        return make_onnx(spec)
    raise SystemExit(f"unknown engine {spec}")


def items_en_pages(limit, part):
    folder = Path(DATA) / "generated" / "ocr_en"
    rows = [json.loads(line) for line in open(folder / "truth.jsonl", encoding="utf-8")]
    rows = [r for r in rows if r["partition"] == part]
    for row in rows[:limit] if limit else rows:
        yield {"id": row["id"], "path": folder / row["file"], "text": row["text"], "group": row["variant"], "identifiers": row["identifiers"]}


def items_urdu(style, limit, part):
    import pyarrow.parquet as pq
    table = pq.read_table(Path(DATA) / "urdu_ocr" / f"{style}-val.parquet")
    names = table.column_names
    tmp = Path("/tmp/urdu_lines")
    tmp.mkdir(exist_ok=True)
    count = 0
    for index in range(table.num_rows):
        key = f"{style}-{index}"
        if partition(key) != part:
            continue
        record = table.slice(index, 1).to_pylist()[0]
        image = record["image"]
        data = image["bytes"] if isinstance(image, dict) else image
        path = tmp / f"{key}.png"
        path.write_bytes(data)
        yield {"id": key, "path": path, "text": record["text"], "group": style, "identifiers": []}
        count += 1
        if limit and count >= limit:
            return


def items_ur(kind, style, limit, part):
    folder = Path(DATA) / "generated" / "ocr_ur"
    rows = [json.loads(line) for line in open(folder / "truth.jsonl", encoding="utf-8")]
    rows = [r for r in rows if r["kind"] == kind and r["partition"] == part and (style in (None, r["style"]))]
    for row in rows[:limit] if limit else rows:
        yield {"id": row["id"], "path": folder / row["file"], "text": row["text"], "group": f"{row['style']}-{row['variant']}", "identifiers": row.get("identifiers", [])}


def items_cord(limit, part):
    import pyarrow.parquet as pq
    table = pq.read_table(Path(DATA) / "cord" / "test.parquet")
    tmp = Path("/tmp/cord")
    tmp.mkdir(exist_ok=True)
    count = 0
    for index in range(table.num_rows):
        key = f"cord-{index}"
        if partition(key) != part:
            continue
        record = table.slice(index, 1).to_pylist()[0]
        truth = json.loads(record["ground_truth"])
        text = " ".join(w["text"] for line in truth.get("valid_line", []) for w in line.get("words", []))
        image = record["image"]
        path = tmp / f"{key}.png"
        path.write_bytes(image["bytes"] if isinstance(image, dict) else image)
        yield {"id": key, "path": path, "text": text, "group": "receipt", "identifiers": []}
        count += 1
        if limit and count >= limit:
            return


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--set", required=True, choices=("en_pages", "urdu_naskh", "urdu_nastaliq", "cord", "ur_lines_naskh", "ur_lines_nastaliq", "ur_pages"))
    parser.add_argument("--engine", action="append", required=True)
    parser.add_argument("--partition", default="dev", choices=("dev", "sealed"))
    parser.add_argument("--limit", type=int, default=0)
    args = parser.parse_args()
    if args.set == "en_pages":
        items = list(items_en_pages(args.limit, args.partition))
    elif args.set == "cord":
        items = list(items_cord(args.limit, args.partition))
    elif args.set == "ur_pages":
        items = list(items_ur("page", None, args.limit, args.partition))
    elif args.set.startswith("ur_lines_"):
        items = list(items_ur("line", args.set.split("_")[2], args.limit, args.partition))
    else:
        items = list(items_urdu(args.set.split("_")[1], args.limit, args.partition))
    print(f"set {args.set} partition {args.partition}: {len(items)} images")
    os.makedirs(os.path.join(DATA, "results", "ocr"), exist_ok=True)
    for spec in args.engine:
        run = make_engine(spec)
        groups, started = {}, time.time()
        detail = []
        for item in items:
            t0 = time.time()
            try:
                text, confidence = run(item["path"])
                failed = ""
            except Exception as error:  # a failure is a result, counted as an empty read
                text, confidence, failed = "", None, str(error)[:100]
            seconds = time.time() - t0
            hit = sum(1 for ident in item["identifiers"] if ident["value"] in text)
            by_type = {}
            for ident in item["identifiers"]:
                entry = by_type.setdefault(ident["type"], [0, 0])
                entry[1] += 1
                entry[0] += 1 if ident["value"] in text else 0
            row = {"id": item["id"], "group": item["group"], "cer": cer(item["text"], text), "f1": bag_f1(item["text"], text), "seconds": seconds,
                   "ids_total": len(item["identifiers"]), "ids_hit": hit, "ids_by_type": by_type, "failed": failed, "confidence": confidence}
            detail.append(row)
            groups.setdefault(item["group"], []).append(row)
        stamp = time.strftime("%Y%m%d-%H%M%S")
        out = os.path.join(DATA, "results", "ocr", f"{args.set}-{args.partition}-{spec.replace(':', '_').replace('+', '-')}-{stamp}.json")
        json.dump({"set": args.set, "partition": args.partition, "engine": spec, "rows": detail}, open(out, "w"), indent=0)
        print(f"\nengine {spec}   total {time.time() - started:.0f} s")
        for group, rows in sorted(groups.items()):
            cers = sorted(r["cer"] for r in rows)
            ids_t, ids_h = sum(r["ids_total"] for r in rows), sum(r["ids_hit"] for r in rows)
            idtext = f"  identifiers {ids_h}/{ids_t} = {100 * ids_h / ids_t:.1f}%" if ids_t else ""
            types = {}
            for r in rows:
                for kind, (h, t) in r.get("ids_by_type", {}).items():
                    types.setdefault(kind, [0, 0])
                    types[kind][0] += h
                    types[kind][1] += t
            if types and group in ("clean", "lowres_jpeg"):
                print("     by type: " + "  ".join(f"{k} {100 * h / t:.0f}%" for k, (h, t) in sorted(types.items())))
            print(f"  {group:14} n={len(rows):3}  CER median {100 * cers[len(cers) // 2]:5.1f}%  mean {100 * sum(cers) / len(cers):5.1f}%  F1 {100 * sum(r['f1'] for r in rows) / len(rows):5.1f}%  {sum(r['seconds'] for r in rows) / len(rows):5.1f} s/img{idtext}  failed {sum(1 for r in rows if r['failed'])}")


if __name__ == "__main__":
    main()
