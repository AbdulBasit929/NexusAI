#!/usr/bin/env python3
"""Why are phone numbers missed on mixed images? (step 2 of the OCR next steps, 2026-11-19.) DEVELOPMENT rows only.

For every development image of the mixed set that carries a planted phone number (up to 40 mixed lines, 40 chats, 20 forms, 20 receipts, in hash order), the PRODUCT processor
(EasyOCR + the Latin re-read, size cap 1400, 4 threads) reads the image once and each planted phone is classified:

  found          the planted string appears verbatim in the reading
  separator      the 11 digits appear in order but with other characters between them (a space, a different dash, a letter)
  digits_wrong   an 11-character window of the reading's digits is within two edits of the planted digits
  reversed       the two halves appear in the other order (for example 6783098-0313)
  split          the digits appear in two different lines
  absent         nothing close

The output file holds, per missed phone, the planted number, the nearest text of the reading and the raw EasyOCR boxes near it (EasyOCR text, Latin re-read text, confidences),
so the cause is read from data and not guessed. Nothing is changed in the product.

    docker run --rm --network none --memory 3500m -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro --entrypoint python nexusai-media-eval-easyocr:20261117 \
        /repo/tools/media_data/diagnose_phones.py --out /data/results/ocr/phone-diagnosis-dev-20261119.json
"""
import argparse
import collections
import hashlib
import json
import os
import re
import sys
from pathlib import Path

try:
    import editdistance
except ImportError:  # the host Python has no editdistance: a small pure-Python version for the offline mode
    class editdistance:  # noqa: N801
        @staticmethod
        def eval(a, b):
            prev = list(range(len(b) + 1))
            for i, ca in enumerate(a, 1):
                cur = [i]
                for j, cb in enumerate(b, 1):
                    cur.append(min(prev[j] + 1, cur[j - 1] + 1, prev[j - 1] + (ca != cb)))
                prev = cur
            return prev[-1]

sys.path.insert(0, "/repo/tools/media_data")
sys.path.insert(0, "/repo/ingestion/forensic_records")
DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
MODEL_DIR = f"{DATA}/models/ocr/easyocr/model"
LIMITS = {"mixline": 40, "receipt": 20}   # where the misses are (offline classification of the acceptance readings: mixed lines and receipts)


def sha256(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def rows():
    out = []
    mixed = [json.loads(line) for line in open(Path(DATA) / "generated" / "ocr_mixed" / "truth.jsonl", encoding="utf-8")]
    seen = collections.Counter()
    for r in sorted((r for r in mixed if r["partition"] == "dev"), key=lambda r: hashlib.sha256(r["id"].encode()).hexdigest()):
        if r["kind"] in LIMITS and seen[r["kind"]] < LIMITS[r["kind"]] and any(i["type"] == "phone" for i in r["identifiers"]):
            seen[r["kind"]] += 1
            out.append(r)
    return out


def classify(phone, lines):
    text = "\n".join(lines)
    if phone in text:
        return "found", ""
    digits = re.sub(r"\D", "", phone)
    text_digits = re.sub(r"\D", "", text)
    flat = re.sub(r"[^\d]+", "|", text)
    if re.search(r"\D*".join(digits), text):
        return "separator", ""
    left, right = phone.split("-")[0], phone.split("-")[-1]
    if re.search(re.escape(right) + r"\D{0,3}" + re.escape(left), text):
        return "reversed", ""
    for i in range(0, max(0, len(text_digits) - len(digits) + 1)):
        if editdistance.eval(text_digits[i:i + len(digits)], digits) <= 2:
            return "digits_wrong", text_digits[i:i + len(digits)]
    parts = [p for p in re.split(r"\D+", text) if p]
    if left in parts or right in parts or any(left in p or right in p for p in parts):
        return "split", ""
    return "absent", ""


def from_runs(paths, out):
    """Offline mode: classify the phones of the mixed images in acceptance run files (accept_router_runner.py output: hyp text and identifiers per image), no model needed."""
    records, tally = [], collections.Counter()
    for path in paths:
        for line in open(path, encoding="utf-8"):
            r = json.loads(line)
            if r["set"] != "mixed":
                continue
            kind = r["group"].split("-")[0]
            lines = r["hyp"].split("\n")
            for ident in r["identifiers"]:
                if ident["type"] != "phone":
                    continue
                cls, near = classify(ident["value"], lines)
                tally[(kind, cls)] += 1
                if cls != "found":
                    records.append({"file": os.path.basename(path), "id": r["row"], "kind": kind, "engine": r.get("engine"), "phone": ident["value"], "class": cls, "nearest_digits": near, "lines": lines})
    json.dump(records, open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    print("| page kind | found | separator | digits_wrong | reversed | split | absent |\n|---|---|---|---|---|---|---|")
    for kind in sorted({k for k, _ in tally}):
        print(f"| {kind} | " + " | ".join(str(tally[(kind, c)]) for c in ("found", "separator", "digits_wrong", "reversed", "split", "absent")) + " |")
    return 0


class RecordingReader:
    """Wraps the EasyOCR reader and keeps the last raw detections (box, text, confidence) so a missed phone can be traced to the box that held it."""

    def __init__(self, reader):
        self.reader, self.last = reader, []

    def readtext(self, image, **kwargs):
        self.last = self.reader.readtext(image, **kwargs)
        return self.last


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--out", required=True)
    ap.add_argument("--from-runs", nargs="+", help="offline mode: classify the mixed images of these acceptance run files instead of reading development images")
    args = ap.parse_args(argv)
    if args.from_runs:
        return from_runs(args.from_runs, args.out)
    os.environ.update({"FORENSIC_EASYOCR_MODEL_DIR": MODEL_DIR, "FORENSIC_EASYOCR_THREADS": "4", "FORENSIC_EASYOCR_MAX_SIDE": "1400",
                       "FORENSIC_EASYOCR_DETECTOR_SHA256": sha256(f"{MODEL_DIR}/craft_mlt_25k.pth"), "FORENSIC_EASYOCR_RECOGNIZER_SHA256": sha256(f"{MODEL_DIR}/arabic.pth")})
    import onnx_text_ocr as engine
    from easyocr_ocr_processor import EasyOcrProcessor
    folder = Path(DATA) / "models" / "ocr" / "latin_PP-OCRv5_mobile_rec_onnx"
    latin = engine.PaddleOnnxRecognizer(folder / "inference.onnx", folder / "inference.yml", arabic_script=False)
    base = EasyOcrProcessor.from_environment()
    recording = RecordingReader(base.reader)
    processor = EasyOcrProcessor(recording, model_hashes=base.model_hashes, maximum_side=1400, latin_recognizer=latin, latin_mode="all")
    records, tally = [], collections.Counter()
    for n, r in enumerate(rows(), 1):
        path = Path(DATA) / "generated" / "ocr_mixed" / r["file"]
        result = processor.process_image(path, source_file=path.name, source_sha256="0" * 64, evidence_id="e", version_id="v")
        lines = [o.payload["raw_text"] for o in sorted(result.observations, key=lambda o: o.payload["reading_order"])]
        for ident in r["identifiers"]:
            if ident["type"] != "phone":
                continue
            kind, near = classify(ident["value"], lines)
            tally[(r["kind"], kind)] += 1
            if kind != "found":
                boxes = [{"x": [round(min(p[0] for p in b)), round(max(p[0] for p in b))], "y": [round(min(p[1] for p in b)), round(max(p[1] for p in b))], "text": t, "conf": round(float(c), 2)}
                         for b, t, c in sorted(recording.last, key=lambda item: min(p[0] for p in item[0]))]
                records.append({"id": r["id"], "kind": r["kind"], "variant": r["variant"], "style": r["style"], "phone": ident["value"], "class": kind, "nearest_digits": near, "lines": lines,
                                "image_size_at_reading": "resized to the 1400 cap", "raw_boxes": boxes,
                                "reread": [e for o in result.observations for e in o.payload.get("latin_reread", [])][:6]})
        if n % 20 == 0:
            print(f"{n} images", flush=True)
    json.dump(records, open(args.out, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    print("\n| page kind | found | separator | digits_wrong | reversed | split | absent |\n|---|---|---|---|---|---|---|")
    for kind in LIMITS:
        row = {c: tally[(kind, c)] for c in ("found", "separator", "digits_wrong", "reversed", "split", "absent")}
        print(f"| {kind} | " + " | ".join(str(v) for v in row.values()) + " |")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
