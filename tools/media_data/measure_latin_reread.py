#!/usr/bin/env python3
"""Choose the Latin re-read rule on DEVELOPMENT rows only (docs/data/ocr-latin-reread-preregistration-20261117.md, sections 3 and 4).

EasyOCR runs once per image (a caching wrapper hands the same detections to three product processors): `none` (no re-read), `all` and `confidence` (the PP-OCRv5 Latin recogniser reads again every
box without Arabic-script characters; see easyocr_ocr_processor.reread_latin_boxes). Rows: the development partitions of the generated Urdu pages (3 per style and degradation = 24) and mixed
images (12 per kind = 60), in hash order. Identifier exactness (the planted string appears verbatim), mean character error per image, and the number of boxes replaced are printed per set and rule.

    docker run --rm --network none --memory 3500m -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro --entrypoint python nexusai-media-eval-easyocr:20261117 \
        /repo/tools/media_data/measure_latin_reread.py --out /data/results/ocr/latin-reread-dev-20261117.json
"""
import argparse
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


def sha256(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def rows():
    out = []
    ur = [json.loads(line) for line in open(Path(DATA) / "generated" / "ocr_ur" / "truth.jsonl", encoding="utf-8")]
    seen = {}
    for r in sorted((r for r in ur if r["partition"] == "dev" and r["kind"] == "page"), key=lambda r: hashlib.sha256(r["id"].encode()).hexdigest()):
        key = (r["style"], r["variant"])
        if seen.get(key, 0) < 3:
            seen[key] = seen.get(key, 0) + 1
            out.append(("ur_pages", "ocr_ur", r))
    mixed = [json.loads(line) for line in open(Path(DATA) / "generated" / "ocr_mixed" / "truth.jsonl", encoding="utf-8")]
    seen = {}
    for r in sorted((r for r in mixed if r["partition"] == "dev"), key=lambda r: hashlib.sha256(r["id"].encode()).hexdigest()):
        if seen.get(r["kind"], 0) < 12:
            seen[r["kind"]] = seen.get(r["kind"], 0) + 1
            out.append(("mixed", "ocr_mixed", r))
    return out


class CachingReader:
    def __init__(self, reader):
        self.reader, self.cache = reader, {}

    def readtext(self, image, **kwargs):
        key = hashlib.md5(image.tobytes()).hexdigest()
        if key not in self.cache:
            self.cache[key] = self.reader.readtext(image, **kwargs)
        return self.cache[key]


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--out", required=True)
    args = ap.parse_args(argv)
    import ocr_eval
    import onnx_text_ocr as engine
    os.environ.update({"FORENSIC_EASYOCR_MODEL_DIR": MODEL_DIR, "FORENSIC_EASYOCR_THREADS": "4", "FORENSIC_EASYOCR_MAX_SIDE": "1400",
                       "FORENSIC_EASYOCR_DETECTOR_SHA256": sha256(f"{MODEL_DIR}/craft_mlt_25k.pth"), "FORENSIC_EASYOCR_RECOGNIZER_SHA256": sha256(f"{MODEL_DIR}/arabic.pth")})
    from easyocr_ocr_processor import EasyOcrProcessor
    base = EasyOcrProcessor.from_environment()
    shared = CachingReader(base.reader)
    folder = Path(DATA) / "models" / "ocr" / "latin_PP-OCRv5_mobile_rec_onnx"
    latin = engine.PaddleOnnxRecognizer(folder / "inference.onnx", folder / "inference.yml", arabic_script=False)
    processors = {
        "none": EasyOcrProcessor(shared, model_hashes=base.model_hashes, maximum_side=1400),
        "all": EasyOcrProcessor(shared, model_hashes=base.model_hashes, maximum_side=1400, latin_recognizer=latin, latin_mode="all"),
        "confidence": EasyOcrProcessor(shared, model_hashes=base.model_hashes, maximum_side=1400, latin_recognizer=latin, latin_mode="confidence"),
    }
    records = []
    for n, (set_name, folder_name, row) in enumerate(rows(), 1):
        path = Path(DATA) / "generated" / folder_name / row["file"]
        record = {"set": set_name, "id": row["id"], "kind": row.get("kind"), "variants": {}}
        for rule, processor in processors.items():
            result = processor.process_image(path, source_file=path.name, source_sha256="0" * 64, evidence_id="e", version_id="v")
            text = "\n".join(o.payload["raw_text"] for o in result.observations)
            hits = {}
            for ident in row["identifiers"]:
                a = hits.setdefault(ident["type"], [0, 0])
                a[1] += 1
                a[0] += ident["value"] in text
            record["variants"][rule] = {"cer": ocr_eval.cer(row["text"], text), "ids": hits, "replaced": (result.metadata.get("latin_reread") or {}).get("boxes_replaced", 0)}
        records.append(record)
        if n % 10 == 0:
            print(f"{n} images", flush=True)
    json.dump(records, open(args.out, "w", encoding="utf-8"), indent=0)
    print("\n| set | rule | images | CER mean | identifiers (hit/total) | by type | boxes replaced |\n|---|---|---|---|---|---|---|")
    for set_name in ("ur_pages", "mixed"):
        part = [r for r in records if r["set"] == set_name]
        for rule in processors:
            hit = sum(a[0] for r in part for a in r["variants"][rule]["ids"].values())
            total = sum(a[1] for r in part for a in r["variants"][rule]["ids"].values())
            by = {}
            for r in part:
                for t, a in r["variants"][rule]["ids"].items():
                    x = by.setdefault(t, [0, 0])
                    x[0] += a[0]
                    x[1] += a[1]
            print(f"| {set_name} | {rule} | {len(part)} | {100 * statistics.mean(r['variants'][rule]['cer'] for r in part):.1f}% | {hit}/{total} = {100 * hit / total:.1f}% | "
                  + ", ".join(f"{t} {v[0]}/{v[1]}" for t, v in sorted(by.items())) + f" | {sum(r['variants'][rule]['replaced'] for r in part)} |")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
