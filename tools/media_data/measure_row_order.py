#!/usr/bin/env python3
"""Confirm the table-row rule on DEVELOPMENT rows (docs/data/ocr-table-row-order-preregistration-20261119.md, section 3).

Every image is forced through EasyOCR with the Latin re-read (the router is bypassed so that bank statements are read by EasyOCR too); the detections are read once and the two orderings
(`current` rule, `rows` = the table-row rule) are computed from the same boxes. Rows: development partitions of the mixed set (statement 24, form 24, chat 24, receipt 24 and mixline 24 of the
images NOT cut by the canvas, see clipped_mixlines.py) and of the Urdu pages (24, three per style and degradation), hash order. Measures per kind: mean character error, bag-of-words F1,
word error rate, identifier exactness, and how many lines the rule fired on.

    docker run --rm --network none --memory 3500m -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro --entrypoint python nexusai-media-eval-easyocr:20261117 \
        /repo/tools/media_data/measure_row_order.py --out /data/results/ocr/row-order-dev-20261119.json
"""
import argparse
import collections
import hashlib
import json
import os
import statistics
import sys
from pathlib import Path

sys.path.insert(0, "/repo/tools/media_data")
sys.path.insert(0, "/repo/ingestion/forensic_records")
DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
MODEL_DIR = f"{DATA}/models/ocr/easyocr/model"
PER_KIND = 24


def sha256(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def rows():
    out = []
    clipped = set(json.load(open(Path(DATA) / "results" / "ocr" / "clipped-mixlines-20261119.json", encoding="utf-8"))["ocr_mixed"]["clipped"])
    mixed = [json.loads(line) for line in open(Path(DATA) / "generated" / "ocr_mixed" / "truth.jsonl", encoding="utf-8")]
    seen = collections.Counter()
    for r in sorted((r for r in mixed if r["partition"] == "dev" and r["id"] not in clipped), key=lambda r: hashlib.sha256(r["id"].encode()).hexdigest()):
        if seen[r["kind"]] < PER_KIND:
            seen[r["kind"]] += 1
            out.append((r["kind"], "ocr_mixed", r))
    ur = [json.loads(line) for line in open(Path(DATA) / "generated" / "ocr_ur" / "truth.jsonl", encoding="utf-8")]
    seen = collections.Counter()
    for r in sorted((r for r in ur if r["partition"] == "dev" and r["kind"] == "page"), key=lambda r: hashlib.sha256(r["id"].encode()).hexdigest()):
        key = (r["style"], r["variant"])
        if seen[key] < 3:
            seen[key] += 1
            out.append(("ur_page", "ocr_ur", r))
    return out


class CachingReader:
    def __init__(self, reader):
        self.reader, self.cache = reader, {}

    def readtext(self, image, **kwargs):
        key = hashlib.md5(image.tobytes()).hexdigest()
        if key not in self.cache:
            self.cache[key] = self.reader.readtext(image, **kwargs)
        return self.cache[key]


def wer(reference, hypothesis, oe, editdistance):
    ref, hyp = oe.words(reference), oe.words(hypothesis)
    return editdistance.eval(ref, hyp) / max(1, len(ref))


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--out", required=True)
    args = ap.parse_args(argv)
    import editdistance
    import ocr_eval as oe
    import onnx_text_ocr as engine
    os.environ.update({"FORENSIC_EASYOCR_MODEL_DIR": MODEL_DIR, "FORENSIC_EASYOCR_THREADS": "4", "FORENSIC_EASYOCR_MAX_SIDE": "1400",
                       "FORENSIC_EASYOCR_DETECTOR_SHA256": sha256(f"{MODEL_DIR}/craft_mlt_25k.pth"), "FORENSIC_EASYOCR_RECOGNIZER_SHA256": sha256(f"{MODEL_DIR}/arabic.pth")})
    from easyocr_ocr_processor import EasyOcrProcessor
    folder = Path(DATA) / "models" / "ocr" / "latin_PP-OCRv5_mobile_rec_onnx"
    latin = engine.PaddleOnnxRecognizer(folder / "inference.onnx", folder / "inference.yml", arabic_script=False)
    base = EasyOcrProcessor.from_environment()
    shared = CachingReader(base.reader)
    processors = {
        "current": EasyOcrProcessor(shared, model_hashes=base.model_hashes, maximum_side=1400, latin_recognizer=latin, latin_mode="all"),
        "rows": EasyOcrProcessor(shared, model_hashes=base.model_hashes, maximum_side=1400, latin_recognizer=latin, latin_mode="all", table_rows=True),
    }
    records = []
    for n, (kind, folder_name, row) in enumerate(rows(), 1):
        path = Path(DATA) / "generated" / folder_name / row["file"]
        record = {"kind": kind, "id": row["id"], "variant": row.get("variant"), "variants": {}}
        for rule, processor in processors.items():
            result = processor.process_image(path, source_file=path.name, source_sha256="0" * 64, evidence_id="e", version_id="v")
            text = "\n".join(o.payload["raw_text"] for o in sorted(result.observations, key=lambda o: o.payload["reading_order"]))
            hit = sum(1 for i in row["identifiers"] if i["value"] in text)
            record["variants"][rule] = {"cer": oe.cer(row["text"], text), "f1": oe.bag_f1(row["text"], text), "wer": wer(row["text"], text, oe, editdistance), "ids_hit": hit, "ids_total": len(row["identifiers"]),
                                        "rows_fired": (result.metadata.get("table_rows") or {}).get("rows_read_left_to_right", 0)}
        records.append(record)
        if n % 20 == 0:
            print(f"{n} images", flush=True)
    json.dump(records, open(args.out, "w", encoding="utf-8"), indent=0)
    print("\n| kind | images | rule | CER mean | F1 | WER | identifiers | lines read as table rows |\n|---|---|---|---|---|---|---|---|")
    for kind in ("statement", "form", "chat", "receipt", "mixline", "ur_page"):
        part = [r for r in records if r["kind"] == kind]
        for rule in processors:
            hit = sum(r["variants"][rule]["ids_hit"] for r in part)
            total = sum(r["variants"][rule]["ids_total"] for r in part)
            print(f"| {kind} | {len(part)} | {rule} | {100 * statistics.mean(r['variants'][rule]['cer'] for r in part):.1f}% | {100 * statistics.mean(r['variants'][rule]['f1'] for r in part):.1f}% | "
                  f"{100 * statistics.mean(r['variants'][rule]['wer'] for r in part):.1f}% | {hit}/{total} | {sum(r['variants'][rule]['rows_fired'] for r in part)} |")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
