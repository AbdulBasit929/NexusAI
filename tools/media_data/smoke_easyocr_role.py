"""Smoke test of the routed OCR role with EasyOCR as the worker builds it (run inside the worker overlay image): configured_media_processor() with the worker's OCR environment and
FORENSIC_OCR_EASYOCR_ENABLED=true, then four DEVELOPMENT images (English page, Urdu page, mixed form, receipt photo) through the real router; and a parity check: on two development images the text of the product
processor equals the text of the lab reader that produced the bake-off numbers (tools/media_data/easyocr_reader.py). Synthetic and public images only.

    docker run --rm --network none -v <models/media>:/models/media:ro -v C:/tmp/nexusai-media-data:/data:ro -v <repo>:/repo:ro --env-file <worker OCR env> \
        --entrypoint python nexusai-forensic-records-worker:<overlay tag> /repo/tools/media_data/smoke_easyocr_role.py
"""
import json
import os
import sys
import time
from pathlib import Path

os.environ["FORENSIC_OCR_EASYOCR_ENABLED"] = "true"
sys.path.insert(0, "/app")
sys.path.insert(0, "/repo/tools/media_data")
os.environ.setdefault("NEXUSAI_MEDIA_DATA", "/data")
import media_pipeline  # noqa: E402

started = time.time()
processor = media_pipeline.configured_media_processor()
role = processor.image_processor
print(f"image processor: {type(role).__name__} {getattr(role, 'processor_revision', '')} built in {time.time() - started:.1f} s; easyocr configured: {role.easyocr is not None}; urdu share {role.urdu_share_threshold}", flush=True)


def first(path_jsonl, test):
    return next(r for r in (json.loads(l) for l in open(path_jsonl, encoding="utf-8")) if r["partition"] == "dev" and test(r))


data = Path("/data/generated")
english = first(data / "ocr_en/truth.jsonl", lambda r: r["variant"] == "clean")
urdu = first(data / "ocr_ur/truth.jsonl", lambda r: r["kind"] == "page" and r["style"] == "naskh" and r["variant"] == "clean")
mixed = first(data / "ocr_mixed/truth.jsonl", lambda r: r["kind"] == "form" and r["variant"] == "clean")
cases = [("english page", data / "ocr_en" / english["file"]), ("urdu page", data / "ocr_ur" / urdu["file"]), ("mixed form", data / "ocr_mixed" / mixed["file"])]
receipt = Path("/data/debug/receipt-1.png")
if receipt.exists():
    cases.append(("receipt photo", receipt))
failed = False
for name, path in cases:
    t0 = time.time()
    result = role.process_image(path, evidence_id="e", version_id="v", source_sha256="0" * 64, source_file=path.name)
    route = result.metadata.get("ocr_route", {})
    line = result.observations[0].payload["raw_text"] if result.observations else ""
    print(f"{name:14} {time.time() - t0:5.1f} s  obs {len(result.observations):3}  engine {route.get('engine_used')}  share {route.get('arabic_script_share')}  tried {route.get('engines_tried')}  first: {line[:50]!r}", flush=True)
    expected = "easyocr-ur+en" if name in ("urdu page",) else None
    if name == "english page" and route.get("engine_used") == "easyocr-ur+en":
        print("  FAIL: an English page was sent to EasyOCR")
        failed = True
    if expected and route.get("engine_used") != expected:
        print(f"  FAIL: expected {expected}")
        failed = True

# parity with the lab reader (same thread count, same cap off): the product line assembly must give the bake-off text
import easyocr_reader  # noqa: E402
from easyocr_ocr_processor import EasyOcrProcessor  # noqa: E402

lab_reader = easyocr_reader.make_reader(["ur", "en"], "/data")
product = role.easyocr
for name, path in cases[1:3]:
    lab_text, _ = easyocr_reader.read_path(lab_reader, path)
    result = product.process_image(path, evidence_id="e", version_id="v", source_sha256="0" * 64, source_file=path.name)
    product_text = "\n".join(o.payload["raw_text"] for o in result.observations)
    same = lab_text == product_text
    print(f"parity {name}: {'IDENTICAL' if same else 'DIFFERENT'} ({len(lab_text)} vs {len(product_text)} characters; product image cap {product.maximum_side})", flush=True)
    if not same and not product.maximum_side:
        failed = True
print("RESULT:", "FAIL" if failed else "ok")
sys.exit(1 if failed else 0)
